package domain

// RemoteInput describes a bounded HTTP(S) input fetched for a single request.
type RemoteInput struct {
	URL             string `json:"url,omitempty" yaml:"url,omitempty"`
	UserAgent       string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	Proxy           string `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds,omitempty" yaml:"cache_ttl_seconds,omitempty"`
}

// ParseRequest describes a single parse invocation.
type ParseRequest struct {
	Format     string            `json:"format" yaml:"format"`
	Content    []byte            `json:"content,omitempty" yaml:"content,omitempty"`
	Remote     *RemoteInput      `json:"remote,omitempty" yaml:"remote,omitempty"`
	Target     string            `json:"target,omitempty" yaml:"target,omitempty"`
	Processors []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	Meta       map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ParseResult struct {
	Nodes  []NodeIR    `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Source *SourceInfo `json:"source,omitempty" yaml:"source,omitempty"`
	Report Report      `json:"report,omitempty" yaml:"report,omitempty"`
}

type SubscriptionPreviewResult struct {
	SubscriptionName string                        `json:"subscription_name" yaml:"subscription_name"`
	Type             SubscriptionType              `json:"type,omitempty" yaml:"type,omitempty"`
	Format           string                        `json:"format,omitempty" yaml:"format,omitempty"`
	BeforeCount      int                           `json:"before_count" yaml:"before_count"`
	AfterCount       int                           `json:"after_count" yaml:"after_count"`
	StatusCounts     map[string]int                `json:"status_counts" yaml:"status_counts"`
	Nodes            []SubscriptionPreviewNodeDiff `json:"nodes" yaml:"nodes"`
	Report           Report                        `json:"report,omitempty" yaml:"report,omitempty"`
}

type SubscriptionPreviewNodeDiff struct {
	Identity    string            `json:"identity" yaml:"identity"`
	Status      string            `json:"status" yaml:"status"`
	Before      *NodeIR           `json:"before,omitempty" yaml:"before,omitempty"`
	After       *NodeIR           `json:"after,omitempty" yaml:"after,omitempty"`
	TargetNames map[string]string `json:"target_names,omitempty" yaml:"target_names,omitempty"`
}

type SubscriptionTrafficRequest struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Refresh bool   `json:"refresh,omitempty" yaml:"refresh,omitempty"`
}

type SubscriptionTrafficResult struct {
	SubscriptionName string                   `json:"subscription_name" yaml:"subscription_name"`
	Type             SubscriptionType         `json:"type,omitempty" yaml:"type,omitempty"`
	Format           string                   `json:"format,omitempty" yaml:"format,omitempty"`
	Cached           bool                     `json:"cached,omitempty" yaml:"cached,omitempty"`
	Traffic          *SubscriptionTrafficItem `json:"traffic,omitempty" yaml:"traffic,omitempty"`
}

// RenderRequest describes a single render invocation.
type RenderRequest struct {
	Format     string          `json:"format" yaml:"format"`
	Nodes      []NodeIR        `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Target     string          `json:"target,omitempty" yaml:"target,omitempty"`
	Processors []ProcessorSpec `json:"processors,omitempty" yaml:"processors,omitempty"`
	Options    RenderOptions   `json:"options,omitempty" yaml:"options,omitempty"`
}

type RenderResult struct {
	ContentType string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Body        []byte `json:"body,omitempty" yaml:"body,omitempty"`
	Report      Report `json:"report,omitempty" yaml:"report,omitempty"`
}

// ConvertRequest describes a direct parse-then-render invocation.
type ConvertRequest struct {
	FromFormat       string            `json:"from_format" yaml:"from_format"`
	ToFormat         string            `json:"to_format" yaml:"to_format"`
	Content          []byte            `json:"content,omitempty" yaml:"content,omitempty"`
	Remote           *RemoteInput      `json:"remote,omitempty" yaml:"remote,omitempty"`
	ParseProcessors  []ProcessorSpec   `json:"parse_processors,omitempty" yaml:"parse_processors,omitempty"`
	RenderProcessors []ProcessorSpec   `json:"render_processors,omitempty" yaml:"render_processors,omitempty"`
	Options          RenderOptions     `json:"options,omitempty" yaml:"options,omitempty"`
	Meta             map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

// FileRequest describes a single file generation request.
// A request may either embed the spec inline (Spec) or refer to a stored spec (Name).
type FileRequest struct {
	Name    string            `json:"name,omitempty" yaml:"name,omitempty"`
	Spec    *FileSpec         `json:"spec,omitempty" yaml:"spec,omitempty"`
	Target  string            `json:"target,omitempty" yaml:"target,omitempty"`
	Request RequestInfo       `json:"request,omitempty" yaml:"request,omitempty"`
	Meta    map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type FileResult struct {
	File        FileDocument `json:"file" yaml:"file"`
	Content     []byte       `json:"content,omitempty" yaml:"content,omitempty"`
	ContentType string       `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Response    ResponseInfo `json:"response,omitempty" yaml:"response,omitempty"`
	Report      Report       `json:"report,omitempty" yaml:"report,omitempty"`
}
