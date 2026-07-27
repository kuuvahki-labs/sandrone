package domain

import "encoding/json"

// RuntimeSettings contains durable service-level defaults used while handling
// requests. Resource-local fields still override these defaults.
type RuntimeSettings struct {
	RemoteDefaults RemoteDefaults `json:"remote_defaults" yaml:"remote_defaults"`
	ProbeDefaults  ProbeDefaults  `json:"probe_defaults" yaml:"probe_defaults"`
	CacheDefaults  CacheDefaults  `json:"cache_defaults" yaml:"cache_defaults"`

	cacheDefaultsSet bool
}

func (s *RuntimeSettings) UnmarshalJSON(data []byte) error {
	var raw struct {
		RemoteDefaults RemoteDefaults `json:"remote_defaults"`
		ProbeDefaults  ProbeDefaults  `json:"probe_defaults"`
		CacheDefaults  *CacheDefaults `json:"cache_defaults"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.RemoteDefaults = raw.RemoteDefaults
	s.ProbeDefaults = raw.ProbeDefaults
	s.CacheDefaults = CacheDefaults{}
	s.cacheDefaultsSet = false
	if raw.CacheDefaults != nil {
		s.CacheDefaults = *raw.CacheDefaults
		s.cacheDefaultsSet = true
	}
	return nil
}

func (s RuntimeSettings) CacheDefaultsSpecified() bool {
	return s.cacheDefaultsSet ||
		s.CacheDefaults.RemoteFetchTTLSeconds != 0 ||
		s.CacheDefaults.SubscriptionTrafficTTLSeconds != 0 ||
		s.CacheDefaults.SubscriptionRenderTTLSeconds != 0 ||
		s.CacheDefaults.FileRenderTTLSeconds != 0
}

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
