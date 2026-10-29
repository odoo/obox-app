package update

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

func TestState_IsFinal(t *testing.T) {
	tests := []struct {
		state   updater.State
		isFinal bool
	}{
		{updater.StateChecking, false},
		{updater.StateDownloading, false},
		{updater.StateVerifying, false},
		{updater.StateInstalling, false},
	}

	for _, tt := range tests {
		if got := IsFinalState(tt.state); got != tt.isFinal {
			t.Errorf("State(%s).IsFinal() = %v, want %v", tt.state, got, tt.isFinal)
		}
		if got := isInProgress(tt.state); got == tt.isFinal {
			t.Errorf("State(%s).IsInProgress() = %v, want %v", tt.state, got, !tt.isFinal)
		}
	}
}
