package printer

import (
	"fmt"

	"epos-proxy/internal/bluetooth"
	"epos-proxy/internal/logger"
)

type BTPrinterInfo struct {
	Id      string
	Address string
	Name    string
}

func (m *Manager) ListBluetoothPrinters() []BTPrinterInfo {
	printers := make([]BTPrinterInfo, 0)
	if m == nil || m.cfg == nil {
		return printers
	}

	btPrinters := m.cfg.GetBluetoothPrinters()
	for _, btCfg := range btPrinters {
		if btCfg.Name == "" {
			btCfg.Name = "BT - " + btCfg.Address
		}

		printers = append(printers, BTPrinterInfo{
			Id:      encodeBluetoothPrinterID(btCfg.Address),
			Address: btCfg.Address,
			Name:    btCfg.Name,
		})
	}
	return printers
}

func (m *Manager) CheckBluetoothPrinter(address string) error {
	if !bluetooth.IsActive() {
		return fmt.Errorf("bluetooth adapter is not available")
	}

	conn, err := bluetooth.Dial(address)
	if err != nil {
		logger.Errorf("Bluetooth printer %s is unreachable: %v", address, err)
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", address, err)
	}

	_ = conn.Close()
	return nil
}

func (m *Manager) AddBluetoothPrinter(p RawPrinter) error {
	logger.Debugf("Adding Bluetooth printer: %s (%s)", p.Address, p.Name)
	norm, err := bluetooth.ValidateAddress(p.Address)
	if err != nil {
		logger.Errorf("Invalid Bluetooth address %s: %v", p.Address, err)
		return err
	}

	if err := m.CheckBluetoothPrinter(norm); err != nil {
		logger.Errorf("Check Bluetooth printer failed for %s: %v", norm, err)
		return err
	}

	if err := m.cfg.AddBluetoothPrinter(norm, p.Name, string(p.Protocol), p.BottomPadding); err != nil {
		logger.Errorf("Failed to save Bluetooth printer: %v", err)
		return fmt.Errorf("failed to save Bluetooth printer: %w", err)
	}

	logger.Debugf("Bluetooth printer added: %s (%s)", norm, p.Name)
	return nil
}

func (m *Manager) RemoveBluetoothPrinter(address string) error {
	logger.Debugf("Removing Bluetooth printer: %s", address)
	norm, err := bluetooth.ValidateAddress(address)
	if err != nil {
		logger.Errorf("Invalid Bluetooth address %s: %v", address, err)
		return err
	}

	if m == nil || m.cfg == nil {
		return fmt.Errorf("config manager is not initialized")
	}

	if err := m.cfg.RemoveBluetoothPrinter(norm); err != nil {
		logger.Errorf("Failed to remove Bluetooth printer: %v", err)
		return fmt.Errorf("failed to remove Bluetooth printer: %w", err)
	}

	logger.Debugf("Bluetooth printer removed successfully: %s", norm)
	return nil
}

func (m *Manager) isBluetoothPrinterConfigured(address string) bool {
	for _, p := range m.cfg.GetBluetoothPrinters() {
		if p.Address == address {
			return true
		}
	}
	return false
}

func (m *Manager) getBluetoothPrinterConfig(printerId string) (Protocol, int) {
	address, ok := decodeBluetoothPrinterID(printerId)
	if !ok {
		return ProtocolESCPOS, 0
	}

	for _, p := range m.cfg.GetBluetoothPrinters() {
		if p.Address == address {
			proto := ParseProtocol(p.Protocol)
			if p.BottomPadding < 0 {
				return proto, 0
			}
			return proto, p.BottomPadding
		}
	}
	return ProtocolESCPOS, 0
}
