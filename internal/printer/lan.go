package printer

import (
	"fmt"
	"net"
	"strings"
	"time"

	"obox-app/internal/config"
	"obox-app/internal/logger"
)

const (
	LANPort           = 9100
	LANConnectTimeout = 3 * time.Second
)

type LANPrinterInfo struct {
	IP string
	Id string
}

func CheckLANPrinter(ip string) error {
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", LANPort))
	conn, err := net.DialTimeout("tcp", addr, LANConnectTimeout)
	if err != nil {
		logger.Debugf("LAN printer %s is offline or unreachable: %v", ip, err)
		return err
	}
	_ = conn.Close()
	logger.Debugf("Successfully connected to LAN printer %s", ip)
	return nil
}

func ListLANPrinters(cfg *config.Manager) []LANPrinterInfo {
	ips := cfg.GetLANPrinters()
	logger.Debugf("Listing %d configured LAN printers", len(ips))
	result := make([]LANPrinterInfo, len(ips))

	for i, ip := range ips {
		result[i] = LANPrinterInfo{
			IP: ip,
			Id: EncodeLANPrinterID(ip),
		}
	}

	return result
}

func ValidateIPAddress(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", fmt.Errorf("IP address cannot be empty")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		logger.Warnf("Invalid IP address format for input: %s", ip)
		return "", fmt.Errorf("invalid IP address format")
	}

	return ip, nil
}
