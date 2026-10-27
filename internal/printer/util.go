package printer

import (
	"obox-app/internal/logger"
	"strconv"
	"strings"

	"github.com/google/gousb"
)

var openDevices = func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
	return ctx.OpenDevices(fn)
}

func pathToString(desc *gousb.DeviceDesc) string {
	parts := make([]string, 0, len(desc.Path)+1)

	// Add bus first
	parts = append(parts, strconv.Itoa(desc.Bus))

	// Convert each path element
	for _, p := range desc.Path {
		parts = append(parts, strconv.Itoa(p))
	}
	logger.Debugf("parts: %s", strings.Join(parts, "."))
	return strings.Join(parts, ".")
}
