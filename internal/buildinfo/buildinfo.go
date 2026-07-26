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
	revision := Revision()
	if revision == "" {
		return Version()
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	return Version() + " (" + revision + ")"
}
