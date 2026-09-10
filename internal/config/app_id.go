package config

import (
	"crypto/rand"
	"math/big"
)

const (
	charsetNumeric     = "0123456789"
	defaultAppIDLength = 10
	appIDPrefix        = "OBOXAPP"
)

func GenerateAppID() string {
	result := make([]byte, defaultAppIDLength)
	max := big.NewInt(int64(len(charsetNumeric)))

	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// Fallback if crypto/rand fails.
			result[i] = charsetNumeric[i%len(charsetNumeric)]
			continue
		}
		result[i] = charsetNumeric[n.Int64()]
	}

	return appIDPrefix + string(result)
}

func (cm *Manager) GetAppID() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.AppID
}
