package printer

import (
	"github.com/google/gousb"
	"obox-app/internal/testutil"
	"testing"
)

func TestPathToString(t *testing.T) {
	// Case 1: Multi-level path
	descMulti := &gousb.DeviceDesc{Bus: 1, Path: []int{2, 3, 4}}
	testutil.ExpectedEqual(t, pathToString(descMulti), "1.2.3.4")

	// Case 2: Single-level path
	descSingle := &gousb.DeviceDesc{Bus: 2, Path: []int{5}}
	testutil.ExpectedEqual(t, pathToString(descSingle), "2.5")

	// Case 3: Empty path (bus only)
	descEmpty := &gousb.DeviceDesc{Bus: 3, Path: nil}
	testutil.ExpectedEqual(t, pathToString(descEmpty), "3")
}
