package domain

// NodeHTTPOptions configures an uncached HTTP request through each input node.
type NodeHTTPOptions struct {
	URL              string            `json:"url" jsonschema:"HTTP(S) destination requested through each input node"`
	Method           string            `json:"method,omitempty" enum:"GET,HEAD" default:"GET"`
	Core             string            `json:"core,omitempty" enum:"sing-box,mihomo" default:"sing-box"`
	Headers          map[string]string `json:"headers,omitempty"`
	TimeoutMS        int               `json:"timeout_ms,omitzero" default:"5000" minimum:"0"`
	Concurrency      int               `json:"concurrency,omitzero" default:"4" minimum:"0"`
	MaxResponseBytes int64             `json:"max_response_bytes,omitzero" default:"65536" minimum:"0"`
}

// NodeHTTPResult is associated with the zero-based index in the requested subset.
// Non-2xx HTTP statuses are responses, not transport errors.
type NodeHTTPResult struct {
	Index      int                 `json:"index"`
	StatusCode int                 `json:"status_code,omitzero"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body"`
	DurationMS int64               `json:"duration_ms"`
	Error      string              `json:"error,omitempty"`
}
