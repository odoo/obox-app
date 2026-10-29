package buildinfo

var (
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "none"
)

type BuildInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	Commit    string `json:"commit"`
	Dev       bool   `json:"dev"`
}

func Get() BuildInfo {
	return BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		Commit:    Commit,
		Dev:       Version == "dev",
	}
}

func GetVersionInfo() string {
	return "Version: " + Version + "\n" + "Build Time: " + BuildTime + "\n" + "Commit: " + Commit
}
