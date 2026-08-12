package domain

type ValidateRequest struct {
	File    *FileSpec         `json:"file,omitempty" yaml:"file,omitempty"`
	Format  string            `json:"format,omitempty" yaml:"format,omitempty"`
	Content []byte            `json:"content,omitempty" yaml:"content,omitempty"`
	Target  string            `json:"target,omitempty" yaml:"target,omitempty"`
	Meta    map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ValidateResult struct {
	OK     bool              `json:"ok" yaml:"ok"`
	Counts ValidationCounts  `json:"counts" yaml:"counts"`
	Issues []ValidationIssue `json:"issues,omitempty" yaml:"issues,omitempty"`
	Report Report            `json:"report,omitempty" yaml:"report,omitempty"`
}

type ValidationIssue struct {
	Severity  string   `json:"severity" yaml:"severity"`
	Stage     string   `json:"stage,omitempty" yaml:"stage,omitempty"`
	Code      string   `json:"code" yaml:"code"`
	Message   string   `json:"message" yaml:"message"`
	NodeIndex *int     `json:"node_index,omitempty" yaml:"node_index,omitempty"`
	NodeID    string   `json:"node_id,omitempty" yaml:"node_id,omitempty"`
	NodeName  string   `json:"node_name,omitempty" yaml:"node_name,omitempty"`
	NodeType  NodeType `json:"node_type,omitempty" yaml:"node_type,omitempty"`
	Field     string   `json:"field,omitempty" yaml:"field,omitempty"`
	Target    string   `json:"target,omitempty" yaml:"target,omitempty"`
}

type ValidationCounts struct {
	Input    int `json:"input" yaml:"input"`
	Valid    int `json:"valid" yaml:"valid"`
	Invalid  int `json:"invalid" yaml:"invalid"`
	Errors   int `json:"error" yaml:"error"`
	Warnings int `json:"warning" yaml:"warning"`
}

type InspectResult struct {
	Formats    InspectFormats    `json:"formats" yaml:"formats"`
	Processors InspectProcessors `json:"processors" yaml:"processors"`
	FileKinds  []FileKind        `json:"file_kinds" yaml:"file_kinds"`
	Probe      InspectProbe      `json:"probe" yaml:"probe"`
	Store      InspectStore      `json:"store" yaml:"store"`
}

type InspectFormats struct {
	Parse  []string `json:"parse" yaml:"parse"`
	Render []string `json:"render" yaml:"render"`
}

type InspectProcessors struct {
	Nodes []string `json:"nodes" yaml:"nodes"`
	File  []string `json:"file" yaml:"file"`
}

type InspectProbe struct {
	Methods  []ProbeMethod         `json:"methods" yaml:"methods"`
	Backends []ProbeBackendSummary `json:"backends" yaml:"backends"`
}

type ProbeBackendSummary struct {
	Method  ProbeMethod `json:"method" yaml:"method"`
	Name    string      `json:"name" yaml:"name"`
	Version string      `json:"version,omitempty" yaml:"version,omitempty"`
	Core    string      `json:"core,omitempty" yaml:"core,omitempty"`
}

type InspectStore struct {
	Configured    bool `json:"configured" yaml:"configured"`
	Subscriptions *int `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Files         *int `json:"files,omitempty" yaml:"files,omitempty"`
}
