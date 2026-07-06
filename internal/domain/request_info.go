package domain

type RequestInfo struct {
	TraceID string            `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	Args    map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
	Meta    map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ResponseInfo struct {
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Status  int               `json:"status,omitempty" yaml:"status,omitempty"`
}
