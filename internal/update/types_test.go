package update

import (
	"testing"
)

func TestState_IsFinal(t *testing.T) {
	tests := []struct {
		state   State
		isFinal bool
	}{
		{StateIdle, true},
		{StateUpdateAvailable, true},
		{StateFailed, true},
		{StateInstalled, true},
		{StateChecking, false},
		{StateDownloading, false},
		{StateInstalling, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsFinal(); got != tt.isFinal {
			t.Errorf("State(%s).IsFinal() = %v, want %v", tt.state, got, tt.isFinal)
		}
		if got := tt.state.IsInProgress(); got == tt.isFinal {
			t.Errorf("State(%s).IsInProgress() = %v, want %v", tt.state, got, !tt.isFinal)
		}
	}
}
