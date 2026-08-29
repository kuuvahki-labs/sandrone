// Package buildinfo exposes the release version and source revision embedded in
// the running Sandrone binary.
package buildinfo

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

//go:embed VERSION
var defaultVersion string

var rawVersion string
var rawRevision string
var rawBuildTime string
var readBuildInfo = debug.ReadBuildInfo

func Version() string {
	value := strings.TrimSpace(rawVersion)
	if value == "" {
		if _, modified := vcsInfo(); modified {
			return "dev"
		}
		value = strings.TrimSpace(defaultVersion)
	}
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		value = "dev"
	}
	return value
}

func DisplayVersion() string {
	version := Version()
	if version == "dev" {
		return version
	}
	return "v" + version
}

func UserAgent() string { return "sandrone/" + Version() }

// BuildTime returns the RFC3339 timestamp injected by the build entrypoint.
func BuildTime() string { return strings.TrimSpace(rawBuildTime) }

func Revision() string {
	if value := strings.TrimSpace(rawRevision); value != "" {
		return value
	}
	revision, modified := vcsInfo()
	if modified {
		return ""
	}
	return revision
}

func vcsInfo() (revision string, modified bool) {
	info, ok := readBuildInfo()
	if !ok {
		return "", false
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = strings.TrimSpace(setting.Value)
		case "vcs.modified":
			modified = strings.EqualFold(strings.TrimSpace(setting.Value), "true")
		}
	}
	return revision, modified
}

func Summary() string {
	var details []string
	revision := Revision()
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		details = append(details, revision)
	}
	if buildTime := BuildTime(); buildTime != "" {
		details = append(details, buildTime)
	}
	if len(details) == 0 {
		return Version()
	}
	return Version() + " (" + strings.Join(details, "; ") + ")"
}
