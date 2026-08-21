package domain

type DiagnoseStatus string

const (
	DiagnoseStatusOK      DiagnoseStatus = "ok"
	DiagnoseStatusPartial DiagnoseStatus = "partial"
	DiagnoseStatusFailed  DiagnoseStatus = "failed"
)

type DiagnoseInputKind string

const (
	DiagnoseInputAuto         DiagnoseInputKind = "auto"
	DiagnoseInputNodes        DiagnoseInputKind = "nodes"
	DiagnoseInputSubscription DiagnoseInputKind = "subscription"
	DiagnoseInputFile         DiagnoseInputKind = "file"
)

// DiagnoseRequest describes one diagnostic execution. Exactly one of Content,
// Remote, SubscriptionName, or File should identify the input.
type DiagnoseRequest struct {
	Kind             DiagnoseInputKind `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	Format           string            `json:"format,omitempty" yaml:"format,omitempty"`
	Content          []byte            `json:"content,omitempty" yaml:"content,omitempty"`
	Remote           *RemoteInput      `json:"remote,omitempty" yaml:"remote,omitempty"`
	SubscriptionName string            `json:"subscription_name,omitempty" yaml:"subscription_name,omitempty"`
	File             *FileRequest      `json:"file,omitempty" yaml:"file,omitempty"`
	Processors       []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	Target           string            `json:"target,omitempty" yaml:"target,omitempty"`
	Meta             map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type DiagnoseInput struct {
	Kind   DiagnoseInputKind `json:"kind" yaml:"kind"`
	Name   string            `json:"name,omitempty" yaml:"name,omitempty"`
	Format string            `json:"format,omitempty" yaml:"format,omitempty"`
	Remote bool              `json:"remote,omitempty" yaml:"remote,omitempty"`
}

type DiagnoseStage struct {
	Index       int           `json:"index" yaml:"index"`
	Scope       string        `json:"scope" yaml:"scope"`
	Kind        string        `json:"kind" yaml:"kind"`
	Type        string        `json:"type,omitempty" yaml:"type,omitempty"`
	Name        string        `json:"name,omitempty" yaml:"name,omitempty"`
	Stage       Stage         `json:"stage,omitempty" yaml:"stage,omitempty"`
	InputCount  int           `json:"input_count" yaml:"input_count"`
	OutputCount int           `json:"output_count" yaml:"output_count"`
	Warnings    []Warning     `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Probes      []ProbeResult `json:"probes,omitempty" yaml:"probes,omitempty"`
	Error       *AppError     `json:"error,omitempty" yaml:"error,omitempty"`
}

type DiagnoseResult struct {
	Status       DiagnoseStatus    `json:"status" yaml:"status"`
	Input        DiagnoseInput     `json:"input" yaml:"input"`
	Stages       []DiagnoseStage   `json:"stages" yaml:"stages"`
	Nodes        []NodeIR          `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	File         *FileDocument     `json:"file,omitempty" yaml:"file,omitempty"`
	Counts       ValidationCounts  `json:"counts" yaml:"counts"`
	Issues       []ValidationIssue `json:"issues,omitempty" yaml:"issues,omitempty"`
	Warnings     []Warning         `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Dependencies []ResourceRef     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	SourceRefs   []SourceRef       `json:"source_refs,omitempty" yaml:"source_refs,omitempty"`
	Report       Report            `json:"report,omitempty" yaml:"report,omitempty"`
	Error        *AppError         `json:"error,omitempty" yaml:"error,omitempty"`
}
