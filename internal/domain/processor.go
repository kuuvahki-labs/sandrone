package domain

import "context"

// NodeProcessor processes a list of NodeIR in the "nodes" stage.
// Implementations must treat the input as read-only and return a new output.
type NodeProcessor interface {
	Name() string
	ApplyNodes(ctx context.Context, in NodeProcessInput) (NodeProcessOutput, error)
}

type NodeProcessInput struct {
	Target  string      `json:"target,omitempty" yaml:"target,omitempty"`
	Nodes   []NodeIR    `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Context NodeContext `json:"context,omitempty" yaml:"context,omitempty"`
	Request RequestInfo `json:"request,omitempty" yaml:"request,omitempty"`
}

type NodeProcessOutput struct {
	Nodes    []NodeIR  `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Warnings []Warning `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type NodeSet struct {
	Nodes        []NodeIR                  `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Dependencies []ResourceRef             `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Sources      []SourceInfo              `json:"sources,omitempty" yaml:"sources,omitempty"`
	Warnings     []Warning                 `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Traffic      []SubscriptionTrafficItem `json:"traffic,omitempty" yaml:"traffic,omitempty"`
	Meta         map[string]string         `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type NodeContext struct {
	InputName    string            `json:"input_name,omitempty" yaml:"input_name,omitempty"`
	Dependencies []ResourceRef     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Sources      []SourceInfo      `json:"sources,omitempty" yaml:"sources,omitempty"`
	Meta         map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

// FileProcessor processes a complete FileDocument and its FileParts.
type FileProcessor interface {
	Name() string
	ApplyFile(ctx context.Context, in FileProcessInput) (FileProcessOutput, error)
}
