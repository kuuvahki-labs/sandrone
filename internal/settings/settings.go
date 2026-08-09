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

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
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
	decoder.DisallowUnknownFields()
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
			Path:                 stored.MCP.Path,
			AllowManagementTools: stored.MCP.AllowManagementTools,
			MaxOutputBytes:       stored.MCP.MaxOutputBytes,
		},
		Log:            stored.Log,
		RemoteDefaults: stored.RemoteDefaults,
		ProbeDefaults:  stored.ProbeDefaults,
		Appearance:     stored.Appearance,
		Subscriptions:  stored.Subscriptions,
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
		stored.LegacyWebUI != nil
	return normalized, removedFields, nil
}

type storedSettings struct {
	SchemaVersion  int                         `json:"schema_version"`
	HTTP           storedHTTPSettings          `json:"http"`
	MCP            storedMCPSettings           `json:"mcp"`
	LegacyWebUI    *storedWebUISettings        `json:"webui,omitempty"`
	Log            domain.LogSettings          `json:"log"`
	RemoteDefaults domain.RemoteDefaults       `json:"remote_defaults"`
	ProbeDefaults  domain.ProbeDefaults        `json:"probe_defaults"`
	CacheDefaults  *domain.CacheDefaults       `json:"cache_defaults"`
	Appearance     domain.AppearanceSettings   `json:"appearance"`
	Subscriptions  domain.SubscriptionSettings `json:"subscriptions"`
}

type storedHTTPSettings struct {
	Listen              string  `json:"listen"`
	LegacyToken         *string `json:"token,omitempty"`
	LegacyTokenRequired *bool   `json:"token_required,omitempty"`
}

type storedMCPSettings struct {
	LegacyTransport      *string `json:"transport,omitempty"`
	Path                 string  `json:"path"`
	AllowManagementTools bool    `json:"allow_management_tools"`
	MaxOutputBytes       int     `json:"max_output_bytes"`
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
			UserAgent: buildinfo.UserAgent(),
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
		CacheDefaults: domain.CacheDefaults{
			SubscriptionTrafficTTLSeconds: 60,
		},
		Appearance: domain.AppearanceSettings{
			ThemeMode: "dark",
			Locale:    "auto",
		},
		Subscriptions: domain.SubscriptionSettings{
			AutoLoadTraffic: false,
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
	out.MCP.AllowManagementTools = value.MCP.AllowManagementTools
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
	out.Subscriptions = value.Subscriptions
	return out, nil
}

func ApplyUpdate(update domain.SettingsUpdate) (domain.Settings, error) {
	next := domain.Settings{
		SchemaVersion:  update.SchemaVersion,
		HTTP:           update.HTTP,
		MCP:            update.MCP,
		Log:            update.Log,
		RemoteDefaults: update.RemoteDefaults,
		ProbeDefaults:  update.ProbeDefaults,
		Appearance:     update.Appearance,
		Subscriptions:  update.Subscriptions,
	}
	next.SpecifyCacheDefaults(update.CacheDefaults)
	return Normalize(next)
}

func View(value domain.Settings) domain.SettingsView {
	return domain.SettingsView{
		SchemaVersion:  value.SchemaVersion,
		HTTP:           value.HTTP,
		MCP:            value.MCP,
		Log:            value.Log,
		RemoteDefaults: value.RemoteDefaults,
		ProbeDefaults:  value.ProbeDefaults,
		CacheDefaults:  value.CacheDefaults,
		Appearance:     value.Appearance,
		Subscriptions:  value.Subscriptions,
	}
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
	if value.TimeoutMS < 0 || value.Attempts < 0 || value.Concurrency < 0 || value.CacheTTLSeconds < 0 {
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
	out.CacheTTLSeconds = value.CacheTTLSeconds
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

func validateHTTP(value domain.HTTPSettings) error {
	_, _, err := net.SplitHostPort(value.Listen)
	if err != nil {
		return invalid("invalid HTTP listen address: %v", err)
	}
	return nil
}

func validateMCP(value domain.MCPSettings) error {
	if !strings.HasPrefix(value.Path, "/") {
		return invalid("MCP path must start with /")
	}
	if value.MaxOutputBytes < 0 {
		return invalid("MCP max_output_bytes must be non-negative")
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
	if value.SubscriptionTrafficTTLSeconds < 0 {
		return invalid("subscription_traffic_ttl_seconds must be non-negative")
	}
	if value.SubscriptionRenderTTLSeconds < 0 {
		return invalid("subscription_render_ttl_seconds must be non-negative")
	}
	if value.FileRenderTTLSeconds < 0 {
		return invalid("file_render_ttl_seconds must be non-negative")
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
