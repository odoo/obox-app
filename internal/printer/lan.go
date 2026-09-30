package printer

import (
	"fmt"
	"net"
	"strings"
	"time"

	"epos-proxy/internal/logger"
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

func (m *Manager) ListLANPrinters() []LANPrinterInfo {
	ips := m.cfg.GetLANPrinters()
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

func (m *Manager) AddLANPrinter(p RawPrinter) error {
	logger.Debugf("Adding LAN printer: %s", p.Address)

	ip, err := ValidateIPAddress(p.Address)
	if err != nil {
		return fmt.Errorf("invalid IP address: %s, error: %v", p.Address, err)
	}

	if err := CheckLANPrinter(ip); err != nil {
		return fmt.Errorf("LAN printer unreachable: %s, error: %v", ip, err)
	}

	if err := m.cfg.AddLanEposPrinter(ip); err != nil {
		return fmt.Errorf("failed to save LAN printer: %s, error: %v", ip, err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)
	return nil
}
