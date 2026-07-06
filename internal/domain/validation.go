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

type InspectRequest struct {
	Target string            `json:"target,omitempty" yaml:"target,omitempty"`
	Meta   map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type InspectResult struct {
	Capabilities map[string]any `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Report       Report         `json:"report,omitempty" yaml:"report,omitempty"`
}
