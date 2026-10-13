package obox

import (
	"fmt"
	"runtime"
	"time"

	"github.com/kbinani/screenshot"

	"obox-app/internal/logger"
	"obox-app/internal/util"
)

var appStartTime = time.Now()

// MockDeviceInfo allows customizing or mocking device information reported to Odoo.
type MockDeviceInfo struct {
	Model           string
	Manufacturer    string
	Processor       string
	OSVersion       string
	APILevel        string
	BuildNumber     string
	Resolution      string
	WebViewProvider string
	WebViewVersion  string
	WebViewUA       string
	BatteryLevel    int
	BatteryCharging bool
	BatteryPlugged  bool
	MemoryUsed      int64
	MemoryTotal     int64
	StorageUsed     int64
	StorageTotal    int64
	Brightness      int
	Volume          int
	SignalLevel     int
}

var defaultMockInfo = MockDeviceInfo{
	Model:           fmt.Sprintf("Obox POS Station (%s)", runtime.GOOS),
	Manufacturer:    "Odoo S.A.",
	Processor:       fmt.Sprintf("%s (%d CPUs)", runtime.GOARCH, runtime.NumCPU()),
	OSVersion:       fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
	APILevel:        "34",
	BuildNumber:     "2.15.0",
	Resolution:      "1920x1080",
	WebViewProvider: "WebKit / Wails",
	WebViewVersion:  "2.15.0",
	WebViewUA:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OboxApp/2.15.0",
	BatteryLevel:    100,
	BatteryCharging: true,
	BatteryPlugged:  true,
	MemoryUsed:      4 * 1024 * 1024 * 1024,   // 4 GB
	MemoryTotal:     16 * 1024 * 1024 * 1024,  // 16 GB
	StorageUsed:     45 * 1024 * 1024 * 1024,  // 45 GB
	StorageTotal:    256 * 1024 * 1024 * 1024, // 256 GB
	Brightness:      100,
	Volume:          80,
	SignalLevel:     4,
}

func getDisplayResolution() string {
	n := screenshot.NumActiveDisplays()
	if n > 0 {
		bounds := screenshot.GetDisplayBounds(0)
		return fmt.Sprintf("%dx%d", bounds.Dx(), bounds.Dy())
	}
	return defaultMockInfo.Resolution
}

// BuildDeviceState constructs the complete state dictionary expected by Odoo's obox.obox model.
func (m *Manager) BuildDeviceState() map[string]interface{} {
	localIP := util.LocalAddr(m.port, m.cfg.IsNetworkPrintingEnabled())
	res := getDisplayResolution()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	memUsed := int64(memStats.Alloc)
	if memUsed < 100*1024*1024 {
		// Minimum baseline for desktop runtime mockup
		memUsed = 512 * 1024 * 1024
	}
	memTotal := defaultMockInfo.MemoryTotal

	state := map[string]interface{}{
		"name":            "OBOX App",
		"ip":              localIP,
		"local_port":      m.port,
		"locale":          "en-US",
		"timezone":        time.Now().Location().String(),
		"is_device_owner": true,

		"hardware": map[string]interface{}{
			"android_id":      m.appID,
			"version":         "2.15.0",
			"model":           defaultMockInfo.Model,
			"manufacturer":    defaultMockInfo.Manufacturer,
			"processor":       defaultMockInfo.Processor,
			"android_version": defaultMockInfo.OSVersion,
			"api_level":       defaultMockInfo.APILevel,
			"android_build":   defaultMockInfo.BuildNumber,
			"resolution":      res,
			"webview": map[string]interface{}{
				"provider": defaultMockInfo.WebViewProvider,
				"version":  defaultMockInfo.WebViewVersion,
				"ua":       defaultMockInfo.WebViewUA,
			},
		},

		"metrics": map[string]interface{}{
			"signal_level": defaultMockInfo.SignalLevel,
			"screen_on":    true,
			"brightness":   defaultMockInfo.Brightness,
			"volume":       defaultMockInfo.Volume,
			"up_since":     appStartTime.Unix(),
			"battery": map[string]interface{}{
				"level":    defaultMockInfo.BatteryLevel,
				"charging": defaultMockInfo.BatteryCharging,
				"plugged":  defaultMockInfo.BatteryPlugged,
			},
			"memory": map[string]interface{}{
				"used":  memUsed,
				"total": memTotal,
			},
			"storage": map[string]interface{}{
				"used":  defaultMockInfo.StorageUsed,
				"total": defaultMockInfo.StorageTotal,
			},
		},

		"network": map[string]interface{}{
			"wifi":       false,
			"ethernet":   true,
			"ssid":       "",
			"ip":         localIP,
			"local_port": m.port,
		},

		"kiosk": map[string]interface{}{
			"enabled":        true,
			"out_of_service": m.IsScreenLocked(),
			"kiosk_pin":      "",
		},
	}

	if appliedVer := m.GetAppliedConfigVersion(); appliedVer != "" {
		state["config_version"] = appliedVer
	}

	m.devMu.Lock()
	devProvider := m.deviceProvider
	m.devMu.Unlock()

	var addedDevices []map[string]interface{}
	if devProvider != nil {
		addedDevices = devProvider()
	} else {
		addedDevices = []map[string]interface{}{}
	}

	state["devices"] = map[string]interface{}{
		"added": addedDevices,
	}

	logger.Infof("[obox] Device state: %v", state)
	return state
}
