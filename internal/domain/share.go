package domain

import "time"

type Share struct {
	ID             string            `json:"id" yaml:"id"`
	Name           string            `json:"name,omitempty" yaml:"name,omitempty"`
	TargetKind     string            `json:"target_kind" yaml:"target_kind"`
	TargetName     string            `json:"target_name" yaml:"target_name"`
	TargetFormat   string            `json:"target_format,omitempty" yaml:"target_format,omitempty"`
	ContentType    string            `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty,omitzero" yaml:"created_at,omitempty"`
	UpdatedAt      time.Time         `json:"updated_at,omitempty,omitzero" yaml:"updated_at,omitempty"`
	ValidFrom      time.Time         `json:"valid_from,omitempty,omitzero" yaml:"valid_from,omitempty"`
	ValidUntil     time.Time         `json:"valid_until,omitempty,omitzero" yaml:"valid_until,omitempty"`
	LastAccessedAt time.Time         `json:"last_accessed_at,omitempty,omitzero" yaml:"last_accessed_at,omitempty"`
	AgeRecipient   string            `json:"age_recipient,omitempty" yaml:"age_recipient,omitempty"`
	MaxUses        int               `json:"max_uses,omitempty" yaml:"max_uses,omitempty"`
	UseCount       int               `json:"use_count,omitempty" yaml:"use_count,omitempty"`
	Meta           map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
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
	MaxUses      int               `json:"max_uses,omitempty" yaml:"max_uses,omitempty"`
	Meta         map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ShareListResult struct {
	Shares []Share `json:"shares" yaml:"shares"`
}

type ShareRenderRequest struct {
	ID      string            `json:"id" yaml:"id"`
	Format  string            `json:"format,omitempty" yaml:"format,omitempty"`
	Request RequestInfo       `json:"request,omitempty" yaml:"request,omitempty"`
	Meta    map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type ShareRenderResult struct {
	ContentType string            `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Body        []byte            `json:"body,omitempty" yaml:"body,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Status      int               `json:"status,omitempty" yaml:"status,omitempty"`
}
