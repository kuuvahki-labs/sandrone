package shared

import (
	"strconv"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// HysteriaV1Obfs returns canonical mode/password values while preserving the
// legacy NodeIR shape that stored a non-URI Hysteria v1 password in Obfs.
func HysteriaV1Obfs(node domain.NodeIR) (string, string) {
	if node.Hysteria == nil {
		return "", ""
	}
	mode := node.Hysteria.Obfs
	password := node.Hysteria.ObfsPassword
	if password == "" && mode != "" && node.SourceFormat != "uri" {
		return "", mode
	}
	return mode, password
}

func ParseMihomoHysteria2HopInterval(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	minimum, maximum, ranged := strings.Cut(value, "-")
	if !ranged {
		maximum = ""
	}
	minimumSeconds, err := strconv.ParseUint(strings.TrimSpace(minimum), 10, 64)
	if err != nil || minimumSeconds > uint64((1<<63-1)/int64(time.Second)) {
		return value, ""
	}
	if minimumSeconds == 0 {
		minimumSeconds = 30
	} else if minimumSeconds < 5 {
		minimumSeconds = 5
	}
	minimumDuration := (time.Duration(minimumSeconds) * time.Second).String()
	if maximum == "" {
		return minimumDuration, ""
	}
	maximumSeconds, err := strconv.ParseUint(strings.TrimSpace(maximum), 10, 64)
	if err != nil || maximumSeconds > uint64((1<<63-1)/int64(time.Second)) {
		return value, ""
	}
	if maximumSeconds < minimumSeconds {
		maximumSeconds = minimumSeconds
	}
	return minimumDuration, (time.Duration(maximumSeconds) * time.Second).String()
}

func FormatMihomoHysteria2HopInterval(minimum, maximum string) (string, error) {
	minimumSeconds, err := positiveWholeSeconds(minimum)
	if err != nil {
		return "", err
	}
	if maximum == "" {
		return strconv.FormatInt(minimumSeconds, 10), nil
	}
	maximumSeconds, err := positiveWholeSeconds(maximum)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(minimumSeconds, 10) + "-" + strconv.FormatInt(maximumSeconds, 10), nil
}

func positiveWholeSeconds(value string) (int64, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 || duration%time.Second != 0 {
		return 0, strconv.ErrSyntax
	}
	return int64(duration / time.Second), nil
}
