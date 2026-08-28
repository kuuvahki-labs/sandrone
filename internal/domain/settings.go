package domain

import (
	"encoding/json"
	"time"
)

type RemoteDefaults struct {
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	Proxy     string `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
}

type ProbeDefaults struct {
	Method      string `json:"method,omitempty" yaml:"method,omitempty"`
	Core        string `json:"core,omitempty" yaml:"core,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	NTPServer   string `json:"ntp_server,omitempty" yaml:"ntp_server,omitempty"`
	TimeoutMS   int    `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	Attempts    int    `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	Concurrency int    `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
}

type ScriptDefaults struct {
	TimeoutMS int `json:"timeout_ms" yaml:"timeout_ms"`
}

type CacheDefaults struct {
	RemoteFetchTTLSeconds          int `json:"remote_fetch_ttl_seconds,omitempty" yaml:"remote_fetch_ttl_seconds,omitempty"`
	ProbeTTLSeconds                int `json:"probe_ttl_seconds,omitempty" yaml:"probe_ttl_seconds,omitempty"`
	SubscriptionSnapshotTTLSeconds int `json:"subscription_snapshot_ttl_seconds,omitempty" yaml:"subscription_snapshot_ttl_seconds,omitempty"`
}

type Settings struct {
	SchemaVersion    int                      `json:"schema_version"`
	HTTP             HTTPSettings             `json:"http"`
	MCP              MCPSettings              `json:"mcp"`
	Log              LogSettings              `json:"log"`
	RemoteDefaults   RemoteDefaults           `json:"remote_defaults"`
	ProbeDefaults    ProbeDefaults            `json:"probe_defaults"`
	ScriptDefaults   ScriptDefaults           `json:"script_defaults"`
	CacheDefaults    CacheDefaults            `json:"cache_defaults"`
	Appearance       AppearanceSettings       `json:"appearance"`
	Subscriptions    SubscriptionSettings     `json:"subscriptions"`
	ScheduledRefresh ScheduledRefreshSettings `json:"scheduled_refresh"`

	cacheDefaultsSet bool
}

func (s *Settings) UnmarshalJSON(data []byte) error {
	type settingsAlias Settings
	var raw struct {
		settingsAlias
		CacheDefaults *CacheDefaults `json:"cache_defaults"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = Settings(raw.settingsAlias)
	s.CacheDefaults = CacheDefaults{}
	s.cacheDefaultsSet = false
	if raw.CacheDefaults != nil {
		s.CacheDefaults = *raw.CacheDefaults
		s.cacheDefaultsSet = true
	}
	return nil
}

func (s Settings) CacheDefaultsSpecified() bool {
	return s.cacheDefaultsSet ||
		s.CacheDefaults.RemoteFetchTTLSeconds != 0 ||
		s.CacheDefaults.ProbeTTLSeconds != 0 ||
		s.CacheDefaults.SubscriptionSnapshotTTLSeconds != 0
}

func (s *Settings) SpecifyCacheDefaults(value CacheDefaults) {
	s.CacheDefaults = value
	s.cacheDefaultsSet = true
}

type HTTPSettings struct {
	Listen string `json:"listen"`
}

type MCPSettings struct {
	Path           string `json:"path"`
	MaxOutputBytes int    `json:"max_output_bytes"`
}

type LogSettings struct {
	Level string `json:"level"`
}

type AppearanceSettings struct {
	ThemeMode string `json:"theme_mode"`
	Locale    string `json:"locale"`
}

type SubscriptionSettings struct {
	AutoLoadTraffic bool `json:"auto_load_traffic"`
}

type ScheduledRefreshTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type ScheduledRefreshSettings struct {
	Enabled  bool                     `json:"enabled"`
	Schedule string                   `json:"schedule"`
	Targets  []ScheduledRefreshTarget `json:"targets"`
}

type SettingsUpdate struct {
	SchemaVersion    int                      `json:"schema_version"`
	HTTP             HTTPSettings             `json:"http"`
	MCP              MCPSettings              `json:"mcp"`
	Log              LogSettings              `json:"log"`
	RemoteDefaults   RemoteDefaults           `json:"remote_defaults"`
	ProbeDefaults    ProbeDefaults            `json:"probe_defaults"`
	ScriptDefaults   ScriptDefaults           `json:"script_defaults"`
	CacheDefaults    CacheDefaults            `json:"cache_defaults"`
	Appearance       AppearanceSettings       `json:"appearance"`
	Subscriptions    SubscriptionSettings     `json:"subscriptions"`
	ScheduledRefresh ScheduledRefreshSettings `json:"scheduled_refresh"`
}

type SettingsView struct {
	SchemaVersion    int                      `json:"schema_version"`
	HTTP             HTTPSettings             `json:"http"`
	MCP              MCPSettings              `json:"mcp"`
	Log              LogSettings              `json:"log"`
	RemoteDefaults   RemoteDefaults           `json:"remote_defaults"`
	ProbeDefaults    ProbeDefaults            `json:"probe_defaults"`
	ScriptDefaults   ScriptDefaults           `json:"script_defaults"`
	CacheDefaults    CacheDefaults            `json:"cache_defaults"`
	Appearance       AppearanceSettings       `json:"appearance"`
	Subscriptions    SubscriptionSettings     `json:"subscriptions"`
	ScheduledRefresh ScheduledRefreshSettings `json:"scheduled_refresh"`
}

type ScheduledRefreshStatus struct {
	Enabled          bool       `json:"enabled"`
	Running          bool       `json:"running"`
	NextRunAt        *time.Time `json:"next_run_at,omitempty"`
	LastStartedAt    *time.Time `json:"last_started_at,omitempty"`
	LastCompletedAt  *time.Time `json:"last_completed_at,omitempty"`
	LastSuccessCount int        `json:"last_success_count"`
	LastFailureCount int        `json:"last_failure_count"`
	SkippedCount     int        `json:"skipped_count"`
	LastSkippedAt    *time.Time `json:"last_skipped_at,omitempty"`
}

type SettingsSnapshot struct {
	Settings        SettingsView      `json:"settings"`
	Effective       SettingsView      `json:"effective"`
	Overrides       map[string]string `json:"overrides"`
	RestartRequired []string          `json:"restart_required"`
}
