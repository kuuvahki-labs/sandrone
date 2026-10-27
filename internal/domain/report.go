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

// RemoteSourceResult describes one source before merged processors run. Index is zero-based.
// FetchStatus is ok/failed/skipped; ParseStatus is ok/failed/skipped.
type RemoteSourceResult struct {
	Index       int       `json:"index" yaml:"index"`
	URL         string    `json:"url" yaml:"url"`
	FetchStatus string    `json:"fetch_status" yaml:"fetch_status"`
	ParseStatus string    `json:"parse_status" yaml:"parse_status"`
	Format      string    `json:"format,omitempty" yaml:"format,omitempty"`
	NodeCount   int       `json:"node_count" yaml:"node_count"`
	Error       *AppError `json:"error,omitzero" yaml:"error,omitempty"`
}

type Report struct {
	SourceResults []RemoteSourceResult `json:"source_results,omitempty" yaml:"source_results,omitempty"`
	Kind          string               `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status        string               `json:"status,omitempty" yaml:"status,omitempty"`
	CreatedAt     time.Time            `json:"created_at,omitzero" yaml:"created_at,omitempty"`
	Lossy         bool                 `json:"lossy,omitzero" yaml:"lossy,omitempty"`
	Refs          []ResourceRef        `json:"refs,omitempty" yaml:"refs,omitempty"`
	Dependencies  []ResourceRef        `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	SourceRefs    []SourceRef          `json:"source_refs,omitempty" yaml:"source_refs,omitempty"`
	Warnings      []Warning            `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Render        RenderReport         `json:"render,omitempty" yaml:"render,omitempty"`
	Probe         *ProbeReport         `json:"probe,omitzero" yaml:"probe,omitempty"`
}
