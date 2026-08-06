package domain

import "time"

type Share struct {
	ID           string            `json:"id" yaml:"id"`
	Name         string            `json:"name,omitempty" yaml:"name,omitempty"`
	TargetKind   string            `json:"target_kind" yaml:"target_kind"`
	TargetName   string            `json:"target_name" yaml:"target_name"`
	TargetFormat string            `json:"target_format,omitempty" yaml:"target_format,omitempty"`
	ContentType  string            `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	CreatedAt    time.Time         `json:"created_at,omitempty,omitzero" yaml:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty,omitzero" yaml:"updated_at,omitempty"`
	ValidFrom    time.Time         `json:"valid_from,omitempty,omitzero" yaml:"valid_from,omitempty"`
	ValidUntil   time.Time         `json:"valid_until,omitempty,omitzero" yaml:"valid_until,omitempty"`
	AgeRecipient string            `json:"age_recipient,omitempty" yaml:"age_recipient,omitempty"`
	Meta         map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ShareCreateRequest struct {
	ID           string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name         string            `json:"name,omitempty" yaml:"name,omitempty"`
	TargetKind   string            `json:"target_kind" yaml:"target_kind"`
	TargetName   string            `json:"target_name" yaml:"target_name"`
	TargetFormat string            `json:"target_format,omitempty" yaml:"target_format,omitempty"`
	ContentType  string            `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	ValidFrom    time.Time         `json:"valid_from,omitempty,omitzero" yaml:"valid_from,omitempty"`
	ValidUntil   time.Time         `json:"valid_until,omitempty,omitzero" yaml:"valid_until,omitempty"`
	AgeRecipient string            `json:"age_recipient,omitempty" yaml:"age_recipient,omitempty"`
	Meta         map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ShareListResult struct {
	Shares        []Share                      `json:"shares" yaml:"shares"`
	Presentations map[string]SharePresentation `json:"-" yaml:"-"`
}

type ShareCreateResult struct {
	Share
	Presentation SharePresentation `json:"-" yaml:"-"`
}

type SharePresentation struct {
	PublicFilename  string            `json:"public_filename" yaml:"public_filename"`
	FormatFilenames map[string]string `json:"format_filenames,omitempty" yaml:"format_filenames,omitempty"`
}

type ShareRenderRequest struct {
	ID                string            `json:"id" yaml:"id"`
	Format            string            `json:"format,omitempty" yaml:"format,omitempty"`
	PresentedFilename string            `json:"presented_filename,omitempty" yaml:"presented_filename,omitempty"`
	Request           RequestInfo       `json:"request,omitempty" yaml:"request,omitempty"`
	Meta              map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ShareRenderResult struct {
	ContentType string            `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Body        []byte            `json:"body,omitempty" yaml:"body,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Status      int               `json:"status,omitempty" yaml:"status,omitempty"`
}
