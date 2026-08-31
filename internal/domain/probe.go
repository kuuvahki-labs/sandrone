package domain

import "time"

type ProbeMethod string

const (
	ProbeTCPConnect ProbeMethod = "tcp_connect"
	ProbeUDPNTP     ProbeMethod = "udp_ntp"
	ProbeURLTest    ProbeMethod = "url_test"
)

const (
	CodeProbeBackendUnavailable ErrorCode = "probe_backend_unavailable"
	CodeProbeCoreUnavailable    ErrorCode = "probe_core_unavailable"
	CodeProbeCoreStartFailed    ErrorCode = "probe_core_start_failed"
	CodeProbeCoreAPIFailed      ErrorCode = "probe_core_api_failed"
	CodeProbeNodeUnsupported    ErrorCode = "probe_node_unsupported"
	CodeProbeInvalidTarget      ErrorCode = "probe_invalid_target"
	CodeProbeTimeout            ErrorCode = "probe_timeout"
	CodeProbeTCPFailed          ErrorCode = "probe_tcp_failed"
	CodeProbeUDPNTPFailed       ErrorCode = "probe_udp_ntp_failed"
)

type ProbeRequest struct {
	Input           NodeInput         `json:"input" yaml:"input"`
	Method          ProbeMethod       `json:"method,omitempty" yaml:"method,omitempty"`
	Core            string            `json:"core,omitempty" yaml:"core,omitempty"`
	URL             string            `json:"url,omitempty" yaml:"url,omitempty"`
	NTPServer       string            `json:"ntp_server,omitempty" yaml:"ntp_server,omitempty"`
	ExpectedStatus  string            `json:"expected_status,omitempty" yaml:"expected_status,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	Attempts        int               `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	Concurrency     int               `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	CacheTTLSeconds int               `json:"cache_ttl_seconds,omitempty" yaml:"cache_ttl_seconds,omitempty"`
	Meta            map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ProbeResult struct {
	Results []NodeProbeResult `json:"results" yaml:"results"`
	Report  Report            `json:"report,omitempty" yaml:"report,omitempty"`
}

type NodeProbeResult struct {
	RuntimeID  string    `json:"runtime_id" yaml:"runtime_id"`
	NodeName   string    `json:"node_name" yaml:"node_name"`
	Method     string    `json:"method" yaml:"method"`
	Target     string    `json:"target,omitempty" yaml:"target,omitempty"`
	Core       string    `json:"core,omitempty" yaml:"core,omitempty"`
	Backend    string    `json:"backend,omitempty" yaml:"backend,omitempty"`
	CacheHit   bool      `json:"cache_hit,omitempty" yaml:"cache_hit,omitempty"`
	Alive      bool      `json:"alive" yaml:"alive"`
	DurationMS int       `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty"`
	CheckedAt  time.Time `json:"checked_at" yaml:"checked_at"`
	ErrorCode  string    `json:"error_code,omitempty" yaml:"error_code,omitempty"`
	Error      string    `json:"error,omitempty" yaml:"error,omitempty"`
}

type ProbeReport struct {
	Backend          string                 `json:"backend,omitempty" yaml:"backend,omitempty"`
	BackendVersion   string                 `json:"backend_version,omitempty" yaml:"backend_version,omitempty"`
	Method           string                 `json:"method,omitempty" yaml:"method,omitempty"`
	Core             string                 `json:"core,omitempty" yaml:"core,omitempty"`
	SuccessCount     int                    `json:"success_count" yaml:"success_count"`
	UnsupportedCount int                    `json:"unsupported_count,omitzero" yaml:"unsupported_count,omitempty"`
	FailureCount     int                    `json:"failure_count" yaml:"failure_count"`
	CacheHitCount    int                    `json:"cache_hit_count,omitempty" yaml:"cache_hit_count,omitempty"`
	ErrorCounts      map[string]int         `json:"error_counts,omitempty" yaml:"error_counts,omitempty"`
	Dimensions       []ProbeReportDimension `json:"dimensions,omitempty" yaml:"dimensions,omitempty"`
}

type ProbeReportDimension struct {
	Method           string         `json:"method,omitempty" yaml:"method,omitempty"`
	Core             string         `json:"core,omitempty" yaml:"core,omitempty"`
	SuccessCount     int            `json:"success_count" yaml:"success_count"`
	UnsupportedCount int            `json:"unsupported_count,omitzero" yaml:"unsupported_count,omitempty"`
	FailureCount     int            `json:"failure_count" yaml:"failure_count"`
	CacheHitCount    int            `json:"cache_hit_count,omitempty" yaml:"cache_hit_count,omitempty"`
	ErrorCounts      map[string]int `json:"error_counts,omitempty" yaml:"error_counts,omitempty"`
}
