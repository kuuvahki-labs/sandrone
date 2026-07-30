package domain

import "encoding/json"

type RemoteDefaults struct {
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	Proxy     string `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
}

type ProbeDefaults struct {
	Method          string `json:"method,omitempty" yaml:"method,omitempty"`
	Core            string `json:"core,omitempty" yaml:"core,omitempty"`
	URL             string `json:"url,omitempty" yaml:"url,omitempty"`
	NTPServer       string `json:"ntp_server,omitempty" yaml:"ntp_server,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	Attempts        int    `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	Concurrency     int    `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds,omitempty" yaml:"cache_ttl_seconds,omitempty"`
}

type CacheDefaults struct {
	RemoteFetchTTLSeconds         int `json:"remote_fetch_ttl_seconds,omitempty" yaml:"remote_fetch_ttl_seconds,omitempty"`
	SubscriptionTrafficTTLSeconds int `json:"subscription_traffic_ttl_seconds,omitempty" yaml:"subscription_traffic_ttl_seconds,omitempty"`
	SubscriptionRenderTTLSeconds  int `json:"subscription_render_ttl_seconds,omitempty" yaml:"subscription_render_ttl_seconds,omitempty"`
	FileRenderTTLSeconds          int `json:"file_render_ttl_seconds,omitempty" yaml:"file_render_ttl_seconds,omitempty"`
}

type Settings struct {
	SchemaVersion  int                  `json:"schema_version"`
	HTTP           HTTPSettings         `json:"http"`
	MCP            MCPSettings          `json:"mcp"`
	WebUI          WebUISettings        `json:"webui"`
	Log            LogSettings          `json:"log"`
	RemoteDefaults RemoteDefaults       `json:"remote_defaults"`
	ProbeDefaults  ProbeDefaults        `json:"probe_defaults"`
	CacheDefaults  CacheDefaults        `json:"cache_defaults"`
	Appearance     AppearanceSettings   `json:"appearance"`
	Subscriptions  SubscriptionSettings `json:"subscriptions"`

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
		s.CacheDefaults.SubscriptionTrafficTTLSeconds != 0 ||
		s.CacheDefaults.SubscriptionRenderTTLSeconds != 0 ||
		s.CacheDefaults.FileRenderTTLSeconds != 0
}

func (s *Settings) SpecifyCacheDefaults(value CacheDefaults) {
	s.CacheDefaults = value
	s.cacheDefaultsSet = true
}

type HTTPSettings struct {
	Listen string `json:"listen"`
}

type MCPSettings struct {
	Path                 string `json:"path"`
	AllowManagementTools bool   `json:"allow_management_tools"`
	MaxOutputBytes       int    `json:"max_output_bytes"`
}

type WebUISettings struct {
	StaticDir string `json:"static_dir"`
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

type SettingsUpdate struct {
	SchemaVersion  int                  `json:"schema_version"`
	HTTP           HTTPSettings         `json:"http"`
	MCP            MCPSettings          `json:"mcp"`
	WebUI          WebUISettings        `json:"webui"`
	Log            LogSettings          `json:"log"`
	RemoteDefaults RemoteDefaults       `json:"remote_defaults"`
	ProbeDefaults  ProbeDefaults        `json:"probe_defaults"`
	CacheDefaults  CacheDefaults        `json:"cache_defaults"`
	Appearance     AppearanceSettings   `json:"appearance"`
	Subscriptions  SubscriptionSettings `json:"subscriptions"`
}

type SettingsView struct {
	SchemaVersion  int                  `json:"schema_version"`
	HTTP           HTTPSettings         `json:"http"`
	MCP            MCPSettings          `json:"mcp"`
	WebUI          WebUISettings        `json:"webui"`
	Log            LogSettings          `json:"log"`
	RemoteDefaults RemoteDefaults       `json:"remote_defaults"`
	ProbeDefaults  ProbeDefaults        `json:"probe_defaults"`
	CacheDefaults  CacheDefaults        `json:"cache_defaults"`
	Appearance     AppearanceSettings   `json:"appearance"`
	Subscriptions  SubscriptionSettings `json:"subscriptions"`
}

type SettingsSnapshot struct {
	Settings        SettingsView      `json:"settings"`
	Effective       SettingsView      `json:"effective"`
	Overrides       map[string]string `json:"overrides"`
	RestartRequired []string          `json:"restart_required"`
}
