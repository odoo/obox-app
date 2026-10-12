package main

type LaunchMode int

const (
	ModeNormal LaunchMode = iota
	ModeServer
	ModeService
)

func (m LaunchMode) String() string {
	switch m {
	case ModeNormal:
		return "Normal"
	case ModeServer:
		return "Server"
	case ModeService:
		return "Service"
	default:
		return "Unknown"
	}
}

func determineLaunchMode(isService bool, forceKiosk bool, kioskEnabled bool) LaunchMode {
	if isService {
		return ModeService
	}
	if forceKiosk || kioskEnabled {
		return ModeServer
	}
	return ModeNormal
}
