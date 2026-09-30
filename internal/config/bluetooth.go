package config

func (cm *Manager) AddBluetoothPrinter(address, name string, protocol string, bottomPadding int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.BluetoothPrinters {
		if existing.Address == address {
			cm.Data.BluetoothPrinters[i].Name = name
			cm.Data.BluetoothPrinters[i].Protocol = protocol
			cm.Data.BluetoothPrinters[i].BottomPadding = bottomPadding
			return cm.saveLocked()
		}
	}
	cm.Data.BluetoothPrinters = append(cm.Data.BluetoothPrinters, BluetoothPrinterConfig{
		Address:       address,
		Name:          name,
		Protocol:      protocol,
		BottomPadding: bottomPadding,
	})
	return cm.saveLocked()
}

func (cm *Manager) RemoveBluetoothPrinter(address string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.BluetoothPrinters {
		if existing.Address == address {
			cm.Data.BluetoothPrinters = append(cm.Data.BluetoothPrinters[:i], cm.Data.BluetoothPrinters[i+1:]...)
			return cm.saveLocked()
		}
	}
	return nil
}

func (cm *Manager) GetBluetoothPrinters() []BluetoothPrinterConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.Data.BluetoothPrinters == nil {
		return []BluetoothPrinterConfig{}
	}

	// Return a copy to avoid races if caller modifies the slice
	result := make([]BluetoothPrinterConfig, len(cm.Data.BluetoothPrinters))
	copy(result, cm.Data.BluetoothPrinters)
	return result
}
