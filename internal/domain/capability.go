package domain

type CapabilityDirection string

const (
	CapabilityDirectionParse  CapabilityDirection = "parse"
	CapabilityDirectionRender CapabilityDirection = "render"
)

type CapabilityFieldStatus string

const (
	CapabilityFieldStatusSupported CapabilityFieldStatus = "supported"
	CapabilityFieldStatusLossy     CapabilityFieldStatus = "lossy"
	CapabilityFieldStatusRawOnly   CapabilityFieldStatus = "raw_only"
)

type CapabilityFieldRef struct {
	IRField   string                `json:"ir_field" yaml:"ir_field"`
	Protocol  string                `json:"protocol" yaml:"protocol"`
	Status    CapabilityFieldStatus `json:"status,omitempty" yaml:"status,omitempty"`
	SourceRef SourceRef             `json:"source_ref" yaml:"source_ref"`
	Notes     string                `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type FormatCapability struct {
	Format     string               `json:"format" yaml:"format"`
	Direction  CapabilityDirection  `json:"direction" yaml:"direction"`
	Types      []NodeType           `json:"types" yaml:"types"`
	Fields     []CapabilityFieldRef `json:"fields,omitempty" yaml:"fields,omitempty"`
	Lossy      []CapabilityFieldRef `json:"lossy,omitempty" yaml:"lossy,omitempty"`
	RawOnly    []CapabilityFieldRef `json:"raw_only,omitempty" yaml:"raw_only,omitempty"`
	Reversible bool                 `json:"reversible" yaml:"reversible"`
}

type FormatCapabilityRequest struct {
	Direction CapabilityDirection `json:"direction" yaml:"direction"`
	Format    string              `json:"format" yaml:"format"`
}

type FormatCapabilityFieldCounts struct {
	Supported int `json:"supported" yaml:"supported"`
	Lossy     int `json:"lossy" yaml:"lossy"`
	RawOnly   int `json:"raw_only" yaml:"raw_only"`
}

type FormatCapabilitySummary struct {
	Direction   CapabilityDirection         `json:"direction" yaml:"direction"`
	Format      string                      `json:"format" yaml:"format"`
	NodeTypes   []NodeType                  `json:"node_types" yaml:"node_types"`
	Reversible  bool                        `json:"reversible" yaml:"reversible"`
	FieldCounts FormatCapabilityFieldCounts `json:"field_counts" yaml:"field_counts"`
	Revisions   []string                    `json:"revisions" yaml:"revisions"`
}

type FormatCapabilityListResult struct {
	Items []FormatCapabilitySummary `json:"items" yaml:"items"`
}
