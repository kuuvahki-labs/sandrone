package buildinfo

import "strings"

const defaultVersion = "0.1.0"

var rawVersion = defaultVersion

func Version() string {
	value := strings.TrimSpace(rawVersion)
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return defaultVersion
	}
	return value
}

func DisplayVersion() string { return "v" + Version() }

func UserAgent() string { return "sandrone/" + Version() }
