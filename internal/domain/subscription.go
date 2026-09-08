package domain

import "time"

type SubscriptionType string

const (
	SubscriptionTypeRemote     SubscriptionType = "remote"
	SubscriptionTypeLocal      SubscriptionType = "local"
	SubscriptionTypeCollection SubscriptionType = "collection"
)

type Subscription struct {
	Name               string            `json:"name" yaml:"name"`
	DisplayName        string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Type               SubscriptionType  `json:"type" yaml:"type"`
	Format             string            `json:"format,omitempty" yaml:"format,omitempty"`
	Content            string            `json:"content,omitempty" yaml:"content,omitempty"`
	Remote             *RemoteInput      `json:"remote,omitzero" yaml:"remote,omitempty"`
	Inputs             []NodeInput       `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors         []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	Nodes              []NodeIR          `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	SnapshotTTLSeconds *int              `json:"snapshot_ttl_seconds,omitzero" yaml:"snapshot_ttl_seconds,omitempty"`
	CreatedAt          time.Time         `json:"created_at,omitzero" yaml:"created_at,omitempty"`
	UpdatedAt          time.Time         `json:"updated_at,omitzero" yaml:"updated_at,omitempty"`
	Meta               map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}
