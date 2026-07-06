package domain

import "time"

type RenderOptions struct {
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
}

type RenderReport struct {
	SuccessCount int       `json:"success_count" yaml:"success_count"`
	LostFields   int       `json:"lost_fields" yaml:"lost_fields"`
	Warnings     []Warning `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type Report struct {
	Kind         string        `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status       string        `json:"status,omitempty" yaml:"status,omitempty"`
	CreatedAt    time.Time     `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Lossy        bool          `json:"lossy,omitempty" yaml:"lossy,omitempty"`
	Refs         []ResourceRef `json:"refs,omitempty" yaml:"refs,omitempty"`
	Dependencies []ResourceRef `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	SourceRefs   []SourceRef   `json:"source_refs,omitempty" yaml:"source_refs,omitempty"`
	Warnings     []Warning     `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Render       RenderReport  `json:"render,omitempty" yaml:"render,omitempty"`
	Probe        *ProbeReport  `json:"probe,omitempty" yaml:"probe,omitempty"`
}
