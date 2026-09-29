package appversion

import "strings"

// Version is replaced from the root VERSION file during Docker builds.
var Version = "dev"

func Current() string {
	version := strings.TrimSpace(Version)
	if version == "" {
		return "dev"
	}
	return version
}
