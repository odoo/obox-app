package config

import (
	"regexp"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestAppID_GenerateDefaultFormat(t *testing.T) {
	appID := GenerateAppID()
	expectedLen := len(appIDPrefix) + defaultAppIDLength
	testutil.ExpectedEqual(t, len(appID), expectedLen)

	match, err := regexp.MatchString(`^OBOXAPP[0-9]{10}$`, appID)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, match)
}

func TestAppID_GetAppID(t *testing.T) {
	cm := &Manager{Data: AppConfig{AppID: "ODOOAPP1234567890"}}
	testutil.ExpectedEqual(t, cm.GetAppID(), "ODOOAPP1234567890")
}
