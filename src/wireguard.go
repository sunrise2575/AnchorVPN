package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

func cidr(network string) string {
	_, subnet, e := net.ParseCIDR(network)
	lg.err("ParseCIDR", e)
	return subnet.String()
}

// GenIP returns random IP range in the specified subnet
func wgGenIP(network string) string {
	_, subnet, e := net.ParseCIDR(network)
	lg.err("ParseCIDR", e)

	seed := rand.NewSource(time.Now().UnixNano())
	machine := rand.New(seed)
	uint32IP := binary.BigEndian.Uint32(subnet.IP) | (machine.Uint32() & (^binary.BigEndian.Uint32(subnet.Mask)))
	binaryIP := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(binaryIP, uint32IP)
	return net.IP(binaryIP).String()
}

// GenKey returns pubkey, privkey
func wgGenKey() (string, string) {
	privKey := supportRun("wg genkey")
	pubKey := supportRunPipe("echo "+privKey, "wg pubkey")
	return pubKey, privKey
}

// ClientConf returns configuration file for a client
func wgClientConf(clientIP string, clientPrivKey string) string {
	serverPubKey := supportFile2Str(jsn.get("server.key.public"))
	serverDomain := jsn.get("server.domain")
	serverPort := jsn.get("server.port")

	vpnRoute := cidr(jsn.get("vpn.route"))
	vpnNetwork := cidr(jsn.get("vpn.network"))

	return fmt.Sprintf(
		`
[Interface]\n" +
	PrivateKey = %v
	Address = %v/32
	[Peer]
	PublicKey = %v 
	AllowedIPs = %v, %v
	Endpoint = %v
	PersistentKeepalive = 1
`,
		clientPrivKey,
		clientIP,
		serverPubKey,
		vpnNetwork, vpnRoute,
		net.JoinHostPort(serverDomain, serverPort))
}

// GenServerSetting function
func wgGenServerSetting(configPath string) string {
	vpnRoute := cidr(jsn.get("vpn.route"))
	vpnNetwork := cidr(jsn.get("vpn.network"))

	str := fmt.Sprintf(
		`
[Interface]
	Address = %v
	SaveConfig = true
	PostUp = iptables -A FORWARD -d %v -j ACCEPT && iptables -A FORWARD -d %v -j ACCEPT
	PreDown = iptables -D FORWARD -d %v -j ACCEPT && iptables -D FORWARD -d %v -j ACCEPT
	ListenPort = %v
	PrivateKey = %v
`,
		jsn.get("vpn.network"),
		vpnRoute, vpnNetwork,
		vpnRoute, vpnNetwork,
		jsn.get("server.port"),
		supportFile2Str(jsn.get("server.key.private")))

	result := make(chan string, 6)

	go jsn.getNestedArray("anchor", result, "key.public", "ip", "route")
	for {
		pubKey, ok := <-result
		if !ok {
			break
		}
		ip, ok := <-result
		if !ok {
			break
		}
		ip2, ok := <-result
		if !ok {
			break
		}

		str += fmt.Sprintf(
			`
[Peer]
PublicKey = %v
AllowedIPs = %v
`,
			pubKey,
			ip)

		result2 := make(chan string, 6)
		go jsn.getSimpleArray(ip2, result2)

		for {
			r, ok := <-result2
			if !ok {
				str += "\n\n"
				break
			}
			str += ", " + r
		}
	}

	return str
}

// GetLatestHandshake function
func wgGetLatestHandshake(serverInterface string, clientPubKey string) time.Time {
	a := func() string {
		tmp := supportRun("wg show " + serverInterface + " latest-handshakes")
		str := strings.Split(tmp, "\n")
		for _, s := range str {
			if strings.Contains(s, clientPubKey) {
				t := strings.Split(s, "\t")
				return t[len(t)-1]
			}
		}
		return "0"
	}()

	result, _ := strconv.ParseInt(a, 10, 64)
	return time.Unix(result, 0)
}
