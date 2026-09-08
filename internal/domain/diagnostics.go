package domain

type Warning struct {
	Code        string              `json:"code" yaml:"code"`
	Message     string              `json:"message" yaml:"message"`
	Node        string              `json:"node,omitempty" yaml:"node,omitempty"`
	NodeIndex   *int                `json:"node_index,omitzero" yaml:"node_index,omitempty"`
	NodeContext *WarningNodeContext `json:"node_context,omitzero" yaml:"node_context,omitempty"`
	Field       string              `json:"field,omitempty" yaml:"field,omitempty"`
	Source      string              `json:"source,omitempty" yaml:"source,omitempty"`
	Target      string              `json:"target,omitempty" yaml:"target,omitempty"`
}

type WarningNodeContext struct {
	Format  string         `json:"format,omitempty" yaml:"format,omitempty"`
	Name    string         `json:"name,omitempty" yaml:"name,omitempty"`
	Type    NodeType       `json:"type,omitempty" yaml:"type,omitempty"`
	Server  string         `json:"server,omitempty" yaml:"server,omitempty"`
	Port    uint16         `json:"port,omitzero" yaml:"port,omitempty"`
	Raw     map[string]any `json:"raw,omitempty" yaml:"raw,omitempty"`
	RawLine string         `json:"raw_line,omitempty" yaml:"raw_line,omitempty"`
	Line    int            `json:"line,omitzero" yaml:"line,omitempty"`
}
