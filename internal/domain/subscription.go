package domain

import "time"

type SubscriptionType string

const (
	SubscriptionTypeRemote     SubscriptionType = "remote"
	SubscriptionTypeLocal      SubscriptionType = "local"
	SubscriptionTypeCollection SubscriptionType = "collection"
)

type Subscription struct {
	Name        string            `json:"name" yaml:"name"`
	DisplayName string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Type        SubscriptionType  `json:"type" yaml:"type"`
	Format      string            `json:"format,omitempty" yaml:"format,omitempty"`
	Content     string            `json:"content,omitempty" yaml:"content,omitempty"`
	Remote      *RemoteInput      `json:"remote,omitempty" yaml:"remote,omitempty"`
	Inputs      []NodeInput       `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors  []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	Nodes       []NodeIR          `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Meta        map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}
