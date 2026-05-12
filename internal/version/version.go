package version

import (
	"runtime/debug"
	"strings"
)

const defaultVersion = "0.0.0-dev"

// Version can be overridden at build time:
//
//	go build -ldflags "-X github.com/umono-cms/cli/internal/version.Version=v1.2.3"
var Version = defaultVersion

func init() {
	if Version != "" && Version != defaultVersion {
		return
	}

	Version = buildInfoVersion()
}

func Display() string {
	if Version == "" {
		return "v" + defaultVersion
	}
	if strings.HasPrefix(Version, "v") || !startsWithDigit(Version) {
		return Version
	}

	return "v" + Version
}

func buildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultVersion
	}

	if usableVersion(info.Main.Version) {
		return info.Main.Version
	}

	revision := buildSetting(info, "vcs.revision")
	if revision == "" {
		return defaultVersion
	}

	if len(revision) > 12 {
		revision = revision[:12]
	}

	version := defaultVersion + "+" + revision
	if buildSetting(info, "vcs.modified") == "true" {
		version += ".dirty"
	}

	return version
}

func usableVersion(version string) bool {
	return version != "" && version != "(devel)"
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}

	return ""
}

func startsWithDigit(value string) bool {
	if value == "" {
		return false
	}

	return value[0] >= '0' && value[0] <= '9'
}
