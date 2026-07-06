package domain

import "time"

type ResourceRef struct {
	Kind string `json:"kind" yaml:"kind"`
	Name string `json:"name" yaml:"name"`
}

type ResourceSummary struct {
	Kind        string            `json:"kind" yaml:"kind"`
	Type        string            `json:"type,omitempty" yaml:"type,omitempty"`
	Name        string            `json:"name" yaml:"name"`
	DisplayName string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Size        int64             `json:"size,omitempty" yaml:"size,omitempty"`
	Format      string            `json:"format,omitempty" yaml:"format,omitempty"`
	Target      string            `json:"target,omitempty" yaml:"target,omitempty"`
	Processors  []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Meta        map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
	Warning     string            `json:"warning,omitempty" yaml:"warning,omitempty"`
}

type ResourceListResult struct {
	Items []ResourceSummary `json:"items" yaml:"items"`
}
