package wg;

import (
	"net"
	"math/rand"
	"encoding/binary"
	"time"
	"support"
	"strconv"
	"strings"
)

func cidr(network string) string {
	_, subnet, e := net.ParseCIDR(network);
	support.LogError("ParseCIDR", e);
	return subnet.String();
}

// GenIP returns random IP range in the specified subnet
func GenIP(network string) string {
	_, subnet, e := net.ParseCIDR(network);
	support.LogError("ParseCIDR", e);

	seed := rand.NewSource(time.Now().UnixNano());
	machine := rand.New(seed);
	uint32IP := binary.BigEndian.Uint32(subnet.IP) | (machine.Uint32() & (^binary.BigEndian.Uint32(subnet.Mask)));
	binaryIP := []byte{0, 0, 0, 0};
	binary.BigEndian.PutUint32(binaryIP, uint32IP);
	return net.IP(binaryIP).String();
}

// GenKey returns pubkey, privkey
func GenKey() (string, string) {
	privKey := support.Run("wg genkey");
	pubKey := support.RunPipe("echo " + privKey, "wg pubkey");
	return pubKey, privKey;
}

// ClientConf returns configuration file for a client
func ClientConf(configPath string, clientIP string, clientPrivKey string) string {
	serverPubKey := support.File2Str(support.JSON(configPath, "server.key.public"));
	serverDomain := support.JSON(configPath, "server.domain");
	serverPort := support.JSON(configPath, "server.port");

	vpnRoute := cidr(support.JSON(configPath, "vpn.route"));
	vpnNetwork := cidr(support.JSON(configPath, "vpn.network"));

	str := 
	"[Interface]\n" +
	"PrivateKey = " + clientPrivKey + "\n" +
	"Address = " + clientIP + "/32" + "\n" +
	"[Peer]\n" +
	"PublicKey = " + serverPubKey + "\n" +
	"AllowedIPs = " + vpnNetwork + ", " + vpnRoute + "\n" +
	"Endpoint = " + net.JoinHostPort(serverDomain, serverPort) + "\n" +
	"PersistentKeepalive = 1";

	return str;
}

// GenServerSetting function
func GenServerSetting(configPath string) string {
	vpnRoute := cidr(support.JSON(configPath, "vpn.route"));
	vpnNetwork := cidr(support.JSON(configPath, "vpn.network"));

	str :=
	"[Interface]\n" +
	"Address = " + support.JSON(configPath, "vpn.network") + "\n" +
	"SaveConfig = true\n" +
	"PostUp = iptables -A FORWARD -d " + vpnRoute + " -j ACCEPT && iptables -A FORWARD -d " + vpnNetwork + " -j ACCEPT\n" +
	"PreDown = iptables -D FORWARD -d " + vpnRoute + " -j ACCEPT && iptables -D FORWARD -d " + vpnNetwork + " -j ACCEPT\n" +
	"ListenPort = " + support.JSON(configPath, "server.port") + "\n" +
	"PrivateKey = " + support.File2Str(support.JSON(configPath, "server.key.private")) + "\n\n";


	result := make(chan string, 6);
	go support.JSONNestedArray(configPath, "anchor", result, "key.public", "ip", "route");
	for {
		pubKey, ok := <-result; if !ok { break; }
		ip, ok := <-result; if !ok { break; }
		ip2, ok := <-result; if !ok { break; }

		str += "[Peer]\n";
		str += "PublicKey = " + pubKey + "\n";
		str += "AllowedIPs = " + ip;

		result2 := make(chan string, 6);
		go support.JSONSimpleArray(ip2, result2);

		for {
			r, ok := <-result2;
			if !ok { str += "\n\n"; break; }
			str += ", " + r;
		}
	}

	return str;
}

// GetLatestHandshake function
func GetLatestHandshake(serverInterface string, clientPubKey string) time.Time {
	a := func () string {
		tmp := support.Run("wg show " + serverInterface + " latest-handshakes");
		str := strings.Split(tmp, "\n")
		for _, s := range str {
			if strings.Contains(s, clientPubKey) {
				t := strings.Split(s, "\t");
				return t[len(t)-1];
			}
		}
		return "0";
	}();

	result, _ := strconv.ParseInt(a, 10, 64);
	return time.Unix(result, 0);
}
