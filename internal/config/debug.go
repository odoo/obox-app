package config

import "time"

func (cm *Manager) ToggleDebugMode() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	enabled := !cm.Data.DebugMode
	if enabled {
		exp := time.Now().Add(DebugModeDuration)
		cm.Data.DebugModeExpiresAt = &exp
	} else {
		cm.Data.DebugModeExpiresAt = nil
	}
	cm.Data.DebugMode = enabled
	return cm.saveLocked()
}

func (cm *Manager) IsDebugMode() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.Data.DebugMode {
		if cm.Data.DebugModeExpiresAt == nil {
			exp := time.Now().Add(DebugModeDuration)
			cm.Data.DebugModeExpiresAt = &exp
			_ = cm.saveLocked()
			return true
		}
		if time.Now().After(*cm.Data.DebugModeExpiresAt) {
			cm.Data.DebugMode = false
			cm.Data.DebugModeExpiresAt = nil
			_ = cm.saveLocked()
			return false
		}
		return true
	}
	return false
}
