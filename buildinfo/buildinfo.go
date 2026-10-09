package buildinfo

var (
	Version   = "local"
	BuildTime = "unknown"
	Commit    = "none"
)

func GetVersionInfo() string {
	return "Version: " + Version + "\n" + "Build Time: " + BuildTime + "\n" + "Commit: " + Commit
}
