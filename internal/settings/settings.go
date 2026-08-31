// Package settings defines the canonical project settings defaults, validation,
// update semantics, and redacted API views.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

const SchemaVersion = 1

func Decode(body []byte) (domain.Settings, error) {
	value, _, err := DecodeStored(body)
	return value, err
}

// DecodeStored decodes persisted settings and reports whether removed fields
// should be erased by rewriting the canonical representation.
func DecodeStored(body []byte) (domain.Settings, bool, error) {
	var stored storedSettings
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&stored); err != nil {
		return domain.Settings{}, false, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err != nil {
			return domain.Settings{}, false, err
		}
		return domain.Settings{}, false, errors.New("settings contain trailing JSON")
	}
	value := domain.Settings{
		SchemaVersion: stored.SchemaVersion,
		HTTP:          domain.HTTPSettings{Listen: stored.HTTP.Listen},
		MCP: domain.MCPSettings{
			Path:           stored.MCP.Path,
			MaxOutputBytes: stored.MCP.MaxOutputBytes,
		},
		Log:              stored.Log,
		RemoteDefaults:   stored.RemoteDefaults,
		ProbeDefaults:    stored.ProbeDefaults,
		ScriptDefaults:   stored.ScriptDefaults,
		Appearance:       stored.Appearance,
		Subscriptions:    stored.Subscriptions,
		ScheduledRefresh: stored.ScheduledRefresh,
	}
	if stored.CacheDefaults != nil {
		value.SpecifyCacheDefaults(*stored.CacheDefaults)
	}
	normalized, err := Normalize(value)
	if err != nil {
		return domain.Settings{}, false, err
	}
	removedFields := stored.HTTP.LegacyToken != nil ||
		stored.HTTP.LegacyTokenRequired != nil ||
		stored.MCP.LegacyTransport != nil ||
		stored.MCP.LegacyAllowManagementTools != nil ||
		stored.LegacyWebUI != nil
	return normalized, removedFields, nil
}

type storedSettings struct {
	SchemaVersion    int                             `json:"schema_version"`
	HTTP             storedHTTPSettings              `json:"http"`
	MCP              storedMCPSettings               `json:"mcp"`
	LegacyWebUI      *storedWebUISettings            `json:"webui,omitempty"`
	Log              domain.LogSettings              `json:"log"`
	RemoteDefaults   domain.RemoteDefaults           `json:"remote_defaults"`
	ProbeDefaults    domain.ProbeDefaults            `json:"probe_defaults"`
	ScriptDefaults   domain.ScriptDefaults           `json:"script_defaults"`
	CacheDefaults    *domain.CacheDefaults           `json:"cache_defaults"`
	Appearance       domain.AppearanceSettings       `json:"appearance"`
	Subscriptions    domain.SubscriptionSettings     `json:"subscriptions"`
	ScheduledRefresh domain.ScheduledRefreshSettings `json:"scheduled_refresh"`
}

type storedHTTPSettings struct {
	Listen              string  `json:"listen"`
	LegacyToken         *string `json:"token,omitempty"`
	LegacyTokenRequired *bool   `json:"token_required,omitempty"`
}

type storedMCPSettings struct {
	LegacyTransport            *string `json:"transport,omitempty"`
	LegacyAllowManagementTools *bool   `json:"allow_management_tools,omitempty"`
	Path                       string  `json:"path"`
	MaxOutputBytes             int     `json:"max_output_bytes"`
}

type storedWebUISettings struct {
	StaticDir string `json:"static_dir"`
}

func Default() domain.Settings {
	return domain.Settings{
		SchemaVersion: SchemaVersion,
		HTTP: domain.HTTPSettings{
			Listen: "127.0.0.1:1137",
		},
		MCP: domain.MCPSettings{
			Path:           "/mcp",
			MaxOutputBytes: 1 << 20,
		},
		Log: domain.LogSettings{Level: "info"},
		RemoteDefaults: domain.RemoteDefaults{
			TimeoutMS: int(fetcher.DefaultTimeout / time.Millisecond),
		},
		ProbeDefaults: domain.ProbeDefaults{
			Method:      string(domain.ProbeURLTest),
			Core:        "sing-box",
			URL:         probe.URLTestTarget(domain.ProbeRequest{Method: domain.ProbeURLTest}),
			NTPServer:   probe.NTPServerFromRequest(domain.ProbeRequest{}),
			TimeoutMS:   5000,
			Attempts:    1,
			Concurrency: 10,
		},
		ScriptDefaults: domain.ScriptDefaults{TimeoutMS: 2000},
		CacheDefaults:  domain.CacheDefaults{},
		Appearance: domain.AppearanceSettings{
			ThemeMode: "dark",
			Locale:    "auto",
		},
		Subscriptions: domain.SubscriptionSettings{
			AutoLoadTraffic: false,
			IgnoredWarnings: []domain.IgnoredWarning{},
		},
		ScheduledRefresh: domain.ScheduledRefreshSettings{
			Schedule: "@every 10m",
			Targets:  []domain.ScheduledRefreshTarget{},
		},
	}
}

func Normalize(value domain.Settings) (domain.Settings, error) {
	if value.SchemaVersion != SchemaVersion {
		return domain.Settings{}, invalid("unsupported settings schema_version %d", value.SchemaVersion)
	}

	defaults := Default()
	out := defaults
	out.SchemaVersion = SchemaVersion

	out.HTTP.Listen = firstNonEmpty(strings.TrimSpace(value.HTTP.Listen), defaults.HTTP.Listen)
	if err := validateHTTP(out.HTTP); err != nil {
		return domain.Settings{}, err
	}

	out.MCP.Path = firstNonEmpty(strings.TrimSpace(value.MCP.Path), defaults.MCP.Path)
	out.MCP.MaxOutputBytes = value.MCP.MaxOutputBytes
	if out.MCP.MaxOutputBytes == 0 {
		out.MCP.MaxOutputBytes = defaults.MCP.MaxOutputBytes
	}
	if err := validateMCP(out.MCP); err != nil {
		return domain.Settings{}, err
	}

	out.Log.Level = firstNonEmpty(strings.ToLower(strings.TrimSpace(value.Log.Level)), defaults.Log.Level)
	if err := validateLogLevel(out.Log.Level); err != nil {
		return domain.Settings{}, err
	}

	var err error
	out.RemoteDefaults, err = normalizeRemoteDefaults(value.RemoteDefaults, defaults.RemoteDefaults)
	if err != nil {
		return domain.Settings{}, err
	}
	out.ProbeDefaults, err = normalizeProbeDefaults(value.ProbeDefaults, defaults.ProbeDefaults)
	if err != nil {
		return domain.Settings{}, err
	}
	out.ScriptDefaults, err = normalizeScriptDefaults(value.ScriptDefaults, defaults.ScriptDefaults)
	if err != nil {
		return domain.Settings{}, err
	}
	if value.CacheDefaultsSpecified() {
		out.SpecifyCacheDefaults(value.CacheDefaults)
	}
	if err := validateCacheDefaults(out.CacheDefaults); err != nil {
		return domain.Settings{}, err
	}

	out.Appearance.ThemeMode = firstNonEmpty(strings.ToLower(strings.TrimSpace(value.Appearance.ThemeMode)), defaults.Appearance.ThemeMode)
	out.Appearance.Locale = firstNonEmpty(strings.TrimSpace(value.Appearance.Locale), defaults.Appearance.Locale)
	if err := validateAppearance(out.Appearance); err != nil {
		return domain.Settings{}, err
	}
	out.Subscriptions, err = normalizeSubscriptionSettings(value.Subscriptions)
	if err != nil {
		return domain.Settings{}, err
	}
	out.ScheduledRefresh, err = normalizeScheduledRefresh(value.ScheduledRefresh, defaults.ScheduledRefresh)
	if err != nil {
		return domain.Settings{}, err
	}
	return out, nil
}

func ApplyUpdate(update domain.SettingsUpdate) (domain.Settings, error) {
	next := domain.Settings{
		SchemaVersion:    update.SchemaVersion,
		HTTP:             update.HTTP,
		MCP:              update.MCP,
		Log:              update.Log,
		RemoteDefaults:   update.RemoteDefaults,
		ProbeDefaults:    update.ProbeDefaults,
		ScriptDefaults:   update.ScriptDefaults,
		Appearance:       update.Appearance,
		Subscriptions:    update.Subscriptions,
		ScheduledRefresh: update.ScheduledRefresh,
	}
	next.SpecifyCacheDefaults(update.CacheDefaults)
	return Normalize(next)
}

func View(value domain.Settings) domain.SettingsView {
	return domain.SettingsView{
		SchemaVersion:    value.SchemaVersion,
		HTTP:             value.HTTP,
		MCP:              value.MCP,
		Log:              value.Log,
		RemoteDefaults:   value.RemoteDefaults,
		ProbeDefaults:    value.ProbeDefaults,
		ScriptDefaults:   value.ScriptDefaults,
		CacheDefaults:    value.CacheDefaults,
		Appearance:       value.Appearance,
		Subscriptions:    value.Subscriptions,
		ScheduledRefresh: value.ScheduledRefresh,
	}
}

func normalizeSubscriptionSettings(value domain.SubscriptionSettings) (domain.SubscriptionSettings, error) {
	out := domain.SubscriptionSettings{
		AutoLoadTraffic: value.AutoLoadTraffic,
		IgnoredWarnings: make([]domain.IgnoredWarning, 0, len(value.IgnoredWarnings)),
	}
	seen := make(map[domain.IgnoredWarning]struct{}, len(value.IgnoredWarnings))
	for _, warning := range value.IgnoredWarnings {
		warning.Code = strings.TrimSpace(warning.Code)
		warning.Field = strings.TrimSpace(warning.Field)
		warning.Source = strings.TrimSpace(warning.Source)
		warning.Target = strings.TrimSpace(warning.Target)
		if warning.Code == "" {
			return domain.SubscriptionSettings{}, invalid("ignored warning code is required")
		}
		if _, ok := seen[warning]; ok {
			return domain.SubscriptionSettings{}, invalid("duplicate ignored warning %q", warning.Code)
		}
		seen[warning] = struct{}{}
		out.IgnoredWarnings = append(out.IgnoredWarnings, warning)
	}
	return out, nil
}

func normalizeScheduledRefresh(value, defaults domain.ScheduledRefreshSettings) (domain.ScheduledRefreshSettings, error) {
	out := domain.ScheduledRefreshSettings{
		Enabled:  value.Enabled,
		Schedule: firstNonEmpty(strings.TrimSpace(value.Schedule), defaults.Schedule),
		Targets:  make([]domain.ScheduledRefreshTarget, 0, len(value.Targets)),
	}
	if strings.HasPrefix(out.Schedule, "CRON_TZ=") || strings.HasPrefix(out.Schedule, "TZ=") {
		return domain.ScheduledRefreshSettings{}, invalid("scheduled refresh schedule must use the server local timezone")
	}
	if strings.HasPrefix(out.Schedule, "@every") {
		parts := strings.Fields(out.Schedule)
		if len(parts) != 2 {
			return domain.ScheduledRefreshSettings{}, invalid("invalid scheduled refresh schedule %q", out.Schedule)
		}
		duration, err := time.ParseDuration(parts[1])
		if err != nil || duration < time.Minute {
			return domain.ScheduledRefreshSettings{}, invalid("scheduled refresh @every interval must be at least 1m")
		}
	}
	if _, err := cron.ParseStandard(out.Schedule); err != nil {
		return domain.ScheduledRefreshSettings{}, invalid("invalid scheduled refresh schedule: %v", err)
	}
	seen := make(map[string]struct{}, len(value.Targets))
	for _, target := range value.Targets {
		target.Kind = strings.TrimSpace(target.Kind)
		target.Name = strings.TrimSpace(target.Name)
		switch target.Kind {
		case "subscription", "file":
		default:
			return domain.ScheduledRefreshSettings{}, invalid("unsupported scheduled refresh target kind %q", target.Kind)
		}
		if target.Name == "" {
			return domain.ScheduledRefreshSettings{}, invalid("scheduled refresh target name is required")
		}
		key := target.Kind + "\x00" + target.Name
		if _, ok := seen[key]; ok {
			return domain.ScheduledRefreshSettings{}, invalid("duplicate scheduled refresh target %s %q", target.Kind, target.Name)
		}
		seen[key] = struct{}{}
		out.Targets = append(out.Targets, target)
	}
	if out.Enabled && len(out.Targets) == 0 {
		return domain.ScheduledRefreshSettings{}, invalid("enabled scheduled refresh requires at least one target")
	}
	return out, nil
}

func normalizeRemoteDefaults(value, defaults domain.RemoteDefaults) (domain.RemoteDefaults, error) {
	if value.TimeoutMS < 0 {
		return domain.RemoteDefaults{}, invalid("remote timeout_ms must be non-negative")
	}
	out := defaults
	if userAgent := strings.TrimSpace(value.UserAgent); userAgent != "" {
		out.UserAgent = userAgent
	}
	if proxyURL := strings.TrimSpace(value.Proxy); proxyURL != "" {
		if err := validateProxyURL(proxyURL); err != nil {
			return domain.RemoteDefaults{}, err
		}
		out.Proxy = proxyURL
	}
	if value.TimeoutMS > 0 {
		out.TimeoutMS = value.TimeoutMS
	}
	return out, nil
}

func normalizeProbeDefaults(value, defaults domain.ProbeDefaults) (domain.ProbeDefaults, error) {
	if value.TimeoutMS < 0 || value.Attempts < 0 || value.Concurrency < 0 {
		return domain.ProbeDefaults{}, invalid("probe numeric defaults must be non-negative")
	}
	out := defaults
	if method := strings.TrimSpace(value.Method); method != "" {
		out.Method = string(probe.NormalizeMethod(domain.ProbeMethod(method)))
	}
	if core := strings.TrimSpace(value.Core); core != "" {
		out.Core = probe.NormalizeCore(core)
	}
	if target := strings.TrimSpace(value.URL); target != "" {
		if err := validateHTTPURL("probe url", target); err != nil {
			return domain.ProbeDefaults{}, err
		}
		out.URL = target
	}
	if server := strings.TrimSpace(value.NTPServer); server != "" {
		out.NTPServer = server
	}
	if value.TimeoutMS > 0 {
		out.TimeoutMS = value.TimeoutMS
	}
	if value.Attempts > 0 {
		out.Attempts = value.Attempts
	}
	if value.Concurrency > 0 {
		out.Concurrency = value.Concurrency
	}
	switch out.Method {
	case string(domain.ProbeTCPConnect), string(domain.ProbeUDPNTP), string(domain.ProbeURLTest):
	default:
		return domain.ProbeDefaults{}, invalid("unsupported probe method %q", out.Method)
	}
	switch out.Core {
	case "mihomo", "sing-box":
	default:
		return domain.ProbeDefaults{}, invalid("unsupported probe core %q", out.Core)
	}
	return out, nil
}

func normalizeScriptDefaults(value, defaults domain.ScriptDefaults) (domain.ScriptDefaults, error) {
	if value.TimeoutMS < 0 {
		return domain.ScriptDefaults{}, invalid("script timeout_ms must be non-negative")
	}
	if value.TimeoutMS == 0 {
		return defaults, nil
	}
	return value, nil
}

func validateHTTP(value domain.HTTPSettings) error {
	_, _, err := net.SplitHostPort(value.Listen)
	if err != nil {
		return invalid("invalid HTTP listen address: %v", err)
	}
	return nil
}

func validateMCP(value domain.MCPSettings) error {
	if err := ValidateMCPPath(value.Path); err != nil {
		return err
	}
	if value.MaxOutputBytes < 0 {
		return invalid("MCP max_output_bytes must be non-negative")
	}
	return nil
}

func ValidateMCPPath(value string) error {
	if value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "/") {
		return invalid("MCP path must start with /")
	}
	switch value {
	case "/", "/healthz", "/version", "/convert":
		return invalid("MCP path %q conflicts with public route", value)
	}
	if value == "/s" || strings.HasPrefix(value, "/s/") {
		return invalid("MCP path %q conflicts with public route", value)
	}
	return nil
}

func validateLogLevel(value string) error {
	switch value {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return invalid("unsupported log level %q", value)
	}
}

func validateAppearance(value domain.AppearanceSettings) error {
	switch value.ThemeMode {
	case "system", "light", "dark":
	default:
		return invalid("unsupported theme_mode %q", value.ThemeMode)
	}
	switch value.Locale {
	case "auto", "zh-CN", "en-US":
	default:
		return invalid("unsupported locale %q", value.Locale)
	}
	return nil
}

func validateCacheDefaults(value domain.CacheDefaults) error {
	if value.RemoteFetchTTLSeconds < 0 {
		return invalid("remote_fetch_ttl_seconds must be non-negative")
	}
	if value.ProbeTTLSeconds < 0 {
		return invalid("probe_ttl_seconds must be non-negative")
	}
	if value.SubscriptionSnapshotTTLSeconds < 0 {
		return invalid("subscription_snapshot_ttl_seconds must be non-negative")
	}
	return nil
}

func validateProxyURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return invalid("parse remote proxy url: %v", err)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
	default:
		return invalid("unsupported remote proxy scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return invalid("remote proxy host is required")
	}
	return nil
}

func validateHTTPURL(label, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return invalid("parse %s: %v", label, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return invalid("%s must use http or https", label)
	}
	if parsed.Host == "" {
		return invalid("%s host is required", label)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func invalid(format string, args ...any) error {
	return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf(format, args...))
}
