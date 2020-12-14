package main

import (
	"flag"
	"fmt"
	"strconv"

	"runtime"

	"time"

	"net/http"

	"github.com/didip/tollbooth"
	"github.com/gorilla/mux"
	"github.com/skip2/go-qrcode"

	"encoding/json"
	"os"
	"os/signal"
)

func initDB(dbPath string) {
	e := db.init(dbPath)
	lg.err("DB failed", e)
	if e != nil {
		os.Exit(1)
	}
	lg.out("DB initialization finished.")
}

func initWG(configPath, iface string) {
	supportRun("wg-quick down " + iface)
	str := wgGenServerSetting(configPath)

	tmp, e := db.getAll()
	lg.err("DB", e)

	for _, c := range tmp {
		str +=
			"[Peer]\n" +
				"PublicKey = " + c.PublicKey + "\n" +
				"AllowedIPs = " + c.IP + "/32\n"
		lg.out("Load IP from DB: " + c.IP)
	}

	supportStr2File("/etc/wireguard/"+iface+".conf", str, 600)
	supportRun("wg-quick up " + iface)

	lg.out("Wireguard initialization finished.")
}

func initPeriodicDeletion(iface string, unusedTime, leaseTime int) {
	go func() {
		for {
			db.deleteTime(iface, time.Duration(unusedTime)*time.Second, time.Duration(leaseTime)*time.Second) // Minimum 120 seconds!
			time.Sleep(time.Second)
		}
	}()

	lg.out("Start periodic deletion. time.unused: " + strconv.Itoa(unusedTime) +
		" (sec), time.lease: " + strconv.Itoa(leaseTime) + " (sec)")
}

func genClientIP(configPath string) (IP, error) {
	var clientIP IP
	for {
		clientIP = wgGenIP(jsn.get("vpn.network"))
		ext, e := db.checkIP(string(clientIP))
		if e != nil {
			return clientIP, e
		}
		if !ext {
			break
		}
	}

	return clientIP, nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	configPath := flag.String("path.config", "./config.json",
		"Config path. Must be JSON format")
	dbPath := flag.String("path.db", "./clients.db",
		"Database file path")
	logPath := flag.String("path.log", "./vpn.log",
		"Program log output")
	unusedTime := flag.Int("time.unused", 180,
		"Delete client that has been issued but unused instantly. Unit is seconds")
	leaseTime := flag.Int("time.lease", 3600*24*3,
		"Delete client that expires lease time. Unit is seconds")
	version := flag.Bool("version", false,
		"Print program version")

	flag.Parse()

	if *version {
		fmt.Println("VPN manager")
		fmt.Println("Version: 1.0.0")
		return
	}

	if *configPath == "" {
		fmt.Println("You should provide config file path.")
		return
	}

	jsn.register(*configPath)
	lg.register(*logPath)

	osSignals := make(chan os.Signal)

	signal.Notify(osSignals, os.Interrupt, os.Kill)
	go func() {
		<-osSignals
		lg.out("Received terminal signal. Goodbye.")
		//db.quit()
		os.Exit(1)
	}()

	lg.out("Start VPN manager")

	defer db.quit()

	initDB(*dbPath)
	iface := jsn.get("server.interface")
	initWG(*configPath, iface)
	initPeriodicDeletion(iface, *unusedTime, *leaseTime)

	router := mux.NewRouter()

	router.Methods("GET").Path("/client-list").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
				SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
				SetMessage("Your request rate is too high. Are you a bot?").
				SetMessageContentType("text/plain; charset=utf-8").
				SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
					go lg.http(http.StatusTooManyRequests, r, "High request rate. Blocked.")
				}),
			func(w http.ResponseWriter, r *http.Request) {
				dbcontents, e := db.getAll()
				if e != nil {
					go func() {
						lg.err("DB", e)
						lg.http(http.StatusInternalServerError, r, "DB GetAll error")
					}()
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				content, e := json.Marshal(dbcontents)
				if e != nil {
					go func() {
						lg.err("encoding/json", e)
						lg.http(http.StatusInternalServerError, r, "JSON Marshall error")
					}()
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				go lg.http(http.StatusOK, r, "")
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write([]byte(content))
			}))

	router.Methods("GET").Path("/qrcode").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
				SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
				SetMessage("Your request rate is too high. Are you a bot?").
				SetMessageContentType("text/plain; charset=utf-8").
				SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
					go lg.http(http.StatusTooManyRequests, r, "High request rate. Blocked.")
				}),
			func(w http.ResponseWriter, r *http.Request) {
				clientIP, e := genClientIP(*configPath)
				if e != nil {
					lg.err("DB", e)
					lg.http(http.StatusInternalServerError, r, "DB CheckIP error")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				pubKey, privKey := wgGenKey()

				now := time.Now()
				p := Profile{
					IP:        clientIP,
					PublicKey: pubKey,
					Created:   now,
					LastSeen:  now,
				}

				serverInterface := jsn.get("server.interface")
				supportRun("wg set " + serverInterface + " peer " + pubKey + " allowed-ips " + clientIP + "/32")
				str := wgClientConf(clientIP, privKey)

				var png []byte

				png, e = qrcode.Encode(str, qrcode.Medium, 512)
				if e != nil {
					lg.err("QRCode Encode", e)
					lg.http(http.StatusInternalServerError, r, "QRCode Encoding error")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				e = db.putIP(p)

				if e != nil {
					lg.err("DB", e)
					lg.http(http.StatusInternalServerError, r, "DB PutIP error")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				go lg.http(http.StatusOK, r, "New IP: "+clientIP)

				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "image/png")
				w.Write(png)
			}))

	router.Methods("GET").Path("/").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
				SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
				SetMessage("Your request rate is too high. Are you a bot?").
				SetMessageContentType("text/plain; charset=utf-8").
				SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
					go lg.http(http.StatusTooManyRequests, r, "High request rate. Blocked.")
				}),
			func(w http.ResponseWriter, r *http.Request) {
				clientIP, e := genClientIP(*configPath)
				if e != nil {
					lg.err("DB", e)
					lg.http(http.StatusInternalServerError, r, "DB CheckIP error")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				pubKey, privKey := wgGenKey()

				now := time.Now()
				p := Profile{
					IP:        clientIP,
					PublicKey: pubKey,
					Created:   now,
					LastSeen:  now,
				}

				serverInterface := jsn.get("server.interface")
				supportRun("wg set " + serverInterface + " peer " + pubKey + " allowed-ips " + clientIP + "/32")
				str := wgClientConf(clientIP, privKey)

				e = db.putIP(p)
				if e != nil {
					lg.err("DB", e)
					lg.http(http.StatusInternalServerError, r, "DB PutIP error")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				go lg.http(http.StatusOK, r, "New IP: "+clientIP)
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Write([]byte(str))
			}))

	http.ListenAndServe(":8000", router)
}
