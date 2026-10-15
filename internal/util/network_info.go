package util

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"obox-app/internal/logger"
)

const LOCALHOST_IP = "127.0.0.1"

type NetworkInfo struct {
	IP             string
	Subnet         string
	Interface      string
	ActiveFirewall string
	Zone           string
}

func getLocalIPv4() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer func() { _ = conn.Close() }()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil
	}
	return addr.IP.To4()
}

func localAddrInfo() (ip net.IP, ipNet *net.IPNet, ifaceName string) {
	ip = getLocalIPv4()
	if ip == nil {
		return nil, nil, ""
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ip, nil, ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			n, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if n.IP.To4() != nil && n.IP.To4().Equal(ip) {
				return ip, n, iface.Name
			}
		}
	}
	return ip, nil, ""
}

// formatSubnet returns the network CIDR string for the given IP and mask,
// e.g. "192.168.1.0/24".
func formatSubnet(ip net.IP, mask net.IPMask) string {
	ones, _ := mask.Size()
	return fmt.Sprintf("%s/%d", ip.Mask(mask), ones)
}

// runCmd runs the named command with args, returning its stdout.
// Returns an error if the command is not found or exits non-zero.
func runCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

func getFirewalldZone(ifaceName string) string {
	out, err := runCmd("firewall-cmd", "--get-zone-of-interface="+ifaceName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func getFirewallManager() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS
	}
	for _, unit := range []string{"ufw", "firewalld", "nftables"} {
		if exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil {
			return unit
		}
	}
	return ""
}

func GetNetworkInfo() NetworkInfo {
	ip, ipNet, ifaceName := localAddrInfo()

	info := NetworkInfo{
		IP:             LOCALHOST_IP,
		Interface:      ifaceName,
		ActiveFirewall: getFirewallManager(),
	}

	if ip != nil {
		info.IP = ip.String()
	}

	if ipNet != nil {
		info.Subnet = formatSubnet(ip, ipNet.Mask)
	}

	if ifaceName != "" && info.ActiveFirewall == "firewalld" {
		info.Zone = getFirewalldZone(ifaceName)
	}

	return info
}

func GetLocalIP(isNetworkEnabled bool) string {
	if !isNetworkEnabled {
		return LOCALHOST_IP
	}

	logger.Debugf("Detecting local LAN IP address...")
	if ip := getLocalIPv4(); ip != nil {
		logger.Debugf("Detected LAN IP via UDP route: %v", ip)
		return ip.String()
	}

	logger.Warnf("UDP dial failed or returned non-IPv4 address, falling back to localhost")
	return LOCALHOST_IP
}
