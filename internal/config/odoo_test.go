package config

import (
	"path/filepath"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestOdooConfig_SetAndPersist(t *testing.T) {
	tempDir := t.TempDir()

	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{Port: 4545},
	}

	testutil.ExpectedFalse(t, cm.HasOdooCredentials())

	err := cm.SetOdooCredentials("http://192.168.1.50:8069", "token-abc-123", "db-uuid-xyz")
	testutil.ExpectedNoError(t, err)

	testutil.ExpectedTrue(t, cm.HasOdooCredentials())
	testutil.ExpectedEqual(t, cm.GetOdooDbURL(), "http://192.168.1.50:8069")

	// Reload in a new manager
	cm2 := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{Port: 4545},
	}

	err = cm2.Load()
	testutil.ExpectedNoError(t, err)

	odooCfg := cm2.GetOdooConfig()
	testutil.ExpectedEqual(t, odooCfg.DBURL, "http://192.168.1.50:8069")
	testutil.ExpectedEqual(t, odooCfg.Token, "token-abc-123")
	testutil.ExpectedEqual(t, odooCfg.DbUUID, "db-uuid-xyz")

	// Clear Odoo config
	err = cm2.ClearOdooConfig()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedFalse(t, cm2.HasOdooCredentials())
}
