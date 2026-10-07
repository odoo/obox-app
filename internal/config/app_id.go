package config

import (
	"crypto/rand"
	"math/big"
	randv2 "math/rand/v2"
)

const (
	charsetNumeric     = "0123456789"
	defaultAppIDLength = 10
	appIDPrefix        = "OBOXAPP"
)

func generateAppID() string {
	result := make([]byte, defaultAppIDLength)
	max := big.NewInt(int64(len(charsetNumeric)))

	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// Fallback to math/rand/v2 if crypto/rand fails to avoid static deterministic IDs
			result[i] = charsetNumeric[randv2.IntN(len(charsetNumeric))]
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
