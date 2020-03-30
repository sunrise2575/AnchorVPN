package main

import (
	"fmt"
	"flag"
	"strconv"

	"runtime"

	"time"

	"net/http"
	"github.com/gorilla/mux"
	"github.com/didip/tollbooth"
	"github.com/skip2/go-qrcode"

	"db"
	"wg"
	"support"
	"encoding/json"
	"os"
	"os/signal"
)

func initDB(dbPath string) {
	e := db.Init(dbPath);
	support.LogError("DB failed", e);
	if e != nil { os.Exit(1); }
	support.Log("DB initialization finished.")
}

func initWG(configPath, iface string) {
	support.Run("wg-quick down " + iface);
	str := wg.GenServerSetting(configPath);

	tmp, e := db.GetAll();
	support.LogError("DB", e);

	for _, c := range tmp {
		str += 
		"[Peer]\n" +
		"PublicKey = " + c.PublicKey + "\n" +
		"AllowedIPs = " + c.IP + "/32\n";
		support.Log("Load IP from DB: " + c.IP)
	}

	support.Str2File("/etc/wireguard/" + iface + ".conf", str, 600);
	support.Run("wg-quick up " + iface);

	support.Log("Wireguard initialization finished.")
}

func initProgram(logPath string, c chan os.Signal) {
	support.SetLogPath(logPath);
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		support.Log("Received terminal signal. Goodbye.");
		db.Quit();
		os.Exit(1);
	}();
	support.Log("Start VPN manager");
}

func initPeriodicDeletion(iface string, unusedTime, leaseTime int) {
	go func() {
		for {
			db.DeleteTime(iface, time.Duration(unusedTime) * time.Second, time.Duration(leaseTime) * time.Second); // Minimum 120 seconds!
			time.Sleep(time.Second);
		}
	}();

	support.Log("Start periodic deletion. time.unused: " + strconv.Itoa(unusedTime) +
		" (sec), time.lease: " + strconv.Itoa(leaseTime) + " (sec)");
}

func genClientIP(configPath string) (db.IP, error) {
	var clientIP db.IP
	for {
		clientIP = wg.GenIP(support.JSON(configPath, "vpn.network"));
		ext, e := db.CheckIP(string(clientIP));
		if e != nil {
			return clientIP, e
		}
		if !ext { break; }
	}

	return clientIP, nil;
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU());

	configPath := flag.String("path.config", "./config.json", "Config path. Must be JSON format");
	dbPath := flag.String("path.db", "./clients.db", "Database file path")
	logPath := flag.String("path.log", "./vpn.log", "Program log output");
	unusedTime := flag.Int("time.unused", 30, "Delete client that has been issued but unused instantly. Unit is seconds");
	leaseTime := flag.Int("time.lease", 3600*24*3, "Delete client that expires lease time. Unit is seconds")
	version := flag.Bool("version", false, "Print program version");

	flag.Parse();

	if *version {
		fmt.Println("VPN manager");
		fmt.Println("Version: 1.0.0");
		return;
	}

	if *configPath == "" {
		fmt.Println("You should provide config file path.");
		return;
	}

	osSignals := make(chan os.Signal)
	initProgram(*logPath, osSignals)
	defer db.Quit();
	initDB(*dbPath);
	iface := support.JSON(*configPath, "server.interface");
	initWG(*configPath, iface)
	initPeriodicDeletion(iface, *unusedTime, *leaseTime);

	router := mux.NewRouter();
	//apirouter := router.PathPrefix("/api/").Subrouter();

	
	//router.Methods("GET").Path("/client-list").HandlerFunc(func(w http.ResponseWriter, r *http.Request)
	/*
	router.Handle("/client-list",
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
			SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
			SetTokenBucketExpirationTTL(time.Minute),
*/
	router.Methods("GET").Path("/client-list").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
			SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
			SetMessage("Your request rate is too high. Are you a bot?").
			SetMessageContentType("text/plain; charset=utf-8").
			SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
				go support.LogHTTP(http.StatusTooManyRequests, r, "High request rate. Blocked.");
			}),
			func(w http.ResponseWriter, r *http.Request) {
		dbcontents, e := db.GetAll();
		if e != nil {
			go func() {
				support.LogError("DB", e);
				support.LogHTTP(http.StatusInternalServerError, r, "DB GetAll error");
			}();
			w.WriteHeader(http.StatusInternalServerError);
			return;
		}

		content, e := json.Marshal(dbcontents)
		if e != nil {
			go func() {
				support.LogError("encoding/json", e);
				support.LogHTTP(http.StatusInternalServerError, r, "JSON Marshall error");
			}();
			w.WriteHeader(http.StatusInternalServerError);
			return;
		}

		go support.LogHTTP(http.StatusOK, r, "");
		w.WriteHeader(http.StatusOK);
		w.Header().Set("Content-Type", "application/json; charset=utf-8");
		w.Write([]byte(content));
	}));

	router.Methods("GET").Path("/qrcode").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
			SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
			SetMessage("Your request rate is too high. Are you a bot?").
			SetMessageContentType("text/plain; charset=utf-8").
			SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
				go support.LogHTTP(http.StatusTooManyRequests, r, "High request rate. Blocked.");
			}),
			func(w http.ResponseWriter, r *http.Request) {
				clientIP, e := genClientIP(*configPath);
				if e != nil {
					support.LogError("DB", e);
					support.LogHTTP(http.StatusInternalServerError, r, "DB CheckIP error");
					w.WriteHeader(http.StatusInternalServerError);
					return;
				}
				pubKey, privKey := wg.GenKey();

				now := time.Now()
				p := db.Profile {
					IP:	clientIP,
					PublicKey: pubKey,
					Created: now,
					LastSeen: now,
				}

				serverInterface := support.JSON(*configPath, "server.interface");
				support.Run("wg set " + serverInterface + " peer " + pubKey + " allowed-ips " + clientIP + "/32");
				str := wg.ClientConf(*configPath, clientIP, privKey);

				var png []byte

				png, e = qrcode.Encode(str, qrcode.Medium, 512);
				if e != nil {
					support.LogError("QRCode Encode", e);
					support.LogHTTP(http.StatusInternalServerError, r, "QRCode Encoding error");
					w.WriteHeader(http.StatusInternalServerError);
					return;
				}

				e = db.PutIP(p)
				if e != nil {
					support.LogError("DB", e);
					support.LogHTTP(http.StatusInternalServerError, r, "DB PutIP error");
					w.WriteHeader(http.StatusInternalServerError);
					return;
				}

				go support.LogHTTP(http.StatusOK, r, "New IP: " + clientIP);

				w.WriteHeader(http.StatusOK);
				w.Header().Set("Content-Type", "image/png");
				w.Write(png);
			}));

	router.Methods("GET").Path("/").Handler(
		tollbooth.LimitFuncHandler(
			tollbooth.NewLimiter(1, nil).
			SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP"}).
			SetMessage("Your request rate is too high. Are you a bot?").
			SetMessageContentType("text/plain; charset=utf-8").
			SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
				go support.LogHTTP(http.StatusTooManyRequests, r, "High request rate. Blocked.");
			}),
			func(w http.ResponseWriter, r *http.Request) {
				clientIP, e := genClientIP(*configPath);
				if e != nil {
					support.LogError("DB", e);
					support.LogHTTP(http.StatusInternalServerError, r, "DB CheckIP error");
					w.WriteHeader(http.StatusInternalServerError);
					return;
				}
				pubKey, privKey := wg.GenKey();

				now := time.Now()
				p := db.Profile {
					IP:	clientIP,
					PublicKey: pubKey,
					Created: now,
					LastSeen: now,
				}

				serverInterface := support.JSON(*configPath, "server.interface");
				support.Run("wg set " + serverInterface + " peer " + pubKey + " allowed-ips " + clientIP + "/32");
				str := wg.ClientConf(*configPath, clientIP, privKey);

				e = db.PutIP(p)
				if e != nil {
					support.LogError("DB", e);
					support.LogHTTP(http.StatusInternalServerError, r, "DB PutIP error");
					w.WriteHeader(http.StatusInternalServerError);
					return;
				}

				go support.LogHTTP(http.StatusOK, r, "New IP: " + clientIP);
				w.WriteHeader(http.StatusOK);
				w.Header().Set("Content-Type", "text/plain; charset=utf-8");
				w.Write([]byte(str));
	}));

	// file
	//router.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("src"))));
	//router.PathPrefix("/aaaa/").Handler(http.StripPrefix("/aaaa", http.FileServer(http.Dir("src"))));

	http.ListenAndServe(":8000", router);
}
