package mcpapi

import (
	"encoding/json"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type processorSpec struct {
	Type   string         `json:"type"`
	Stage  domain.Stage   `json:"stage,omitempty"`
	Name   string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

func (spec processorSpec) domain() (domain.ProcessorSpec, error) {
	params, err := rawObject(spec.Params)
	if err != nil {
		return domain.ProcessorSpec{}, fmt.Errorf("processor params: %w", err)
	}
	return domain.ProcessorSpec{
		Type:   spec.Type,
		Stage:  spec.Stage,
		Name:   spec.Name,
		Params: params,
	}, nil
}

func processorSpecsDomain(specs []processorSpec) ([]domain.ProcessorSpec, error) {
	out := make([]domain.ProcessorSpec, len(specs))
	for index, spec := range specs {
		converted, err := spec.domain()
		if err != nil {
			return nil, fmt.Errorf("processor %d: %w", index, err)
		}
		out[index] = converted
	}
	return out, nil
}

func rawObject(values map[string]any) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		body, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = body
	}
	return out, nil
}

type fileConfig struct {
	Subscriptions []string       `json:"subscriptions,omitempty"`
	Settings      map[string]any `json:"settings,omitempty"`
}

func (config fileConfig) domain() (*domain.FileConfig, error) {
	var settings json.RawMessage
	if config.Settings != nil {
		body, err := json.Marshal(config.Settings)
		if err != nil {
			return nil, fmt.Errorf("config.settings: %w", err)
		}
		settings = body
	}
	return &domain.FileConfig{
		Subscriptions: config.Subscriptions,
		Settings:      settings,
	}, nil
}

type fileSpec struct {
	Name                  string            `json:"name"`
	DisplayName           string            `json:"display_name,omitempty"`
	Kind                  domain.FileKind   `json:"kind"`
	Source                domain.FileSource `json:"source"`
	Config                *fileConfig       `json:"config,omitempty"`
	Processors            []processorSpec   `json:"processors,omitempty"`
	RenderCacheTTLSeconds *int              `json:"render_cache_ttl_seconds,omitempty"`
	Meta                  map[string]string `json:"meta,omitempty"`
}

func (spec fileSpec) domain() (domain.FileSpec, error) {
	processors, err := processorSpecsDomain(spec.Processors)
	if err != nil {
		return domain.FileSpec{}, err
	}
	var config *domain.FileConfig
	if spec.Config != nil {
		config, err = spec.Config.domain()
		if err != nil {
			return domain.FileSpec{}, err
		}
	}
	return domain.FileSpec{
		Name:                  spec.Name,
		DisplayName:           spec.DisplayName,
		Kind:                  spec.Kind,
		Source:                spec.Source,
		Config:                config,
		Processors:            processors,
		RenderCacheTTLSeconds: spec.RenderCacheTTLSeconds,
		Meta:                  spec.Meta,
	}, nil
}

type subscription struct {
	Name                  string                  `json:"name"`
	DisplayName           string                  `json:"display_name,omitempty"`
	Type                  domain.SubscriptionType `json:"type"`
	Format                string                  `json:"format,omitempty"`
	Content               string                  `json:"content,omitempty"`
	Remote                *domain.RemoteInput     `json:"remote,omitempty"`
	Inputs                []domain.NodeInput      `json:"inputs,omitempty"`
	Processors            []processorSpec         `json:"processors,omitempty"`
	Nodes                 []domain.NodeIR         `json:"nodes,omitempty"`
	RenderCacheTTLSeconds *int                    `json:"render_cache_ttl_seconds,omitempty"`
	Meta                  map[string]string       `json:"meta,omitempty"`
}

func (sub subscription) domain() (domain.Subscription, error) {
	processors, err := processorSpecsDomain(sub.Processors)
	if err != nil {
		return domain.Subscription{}, err
	}
	return domain.Subscription{
		Name:                  sub.Name,
		DisplayName:           sub.DisplayName,
		Type:                  sub.Type,
		Format:                sub.Format,
		Content:               sub.Content,
		Remote:                sub.Remote,
		Inputs:                sub.Inputs,
		Processors:            processors,
		Nodes:                 sub.Nodes,
		RenderCacheTTLSeconds: sub.RenderCacheTTLSeconds,
		Meta:                  sub.Meta,
	}, nil
}

type convertInput struct {
	FromFormat       string               `json:"from_format"`
	ToFormat         string               `json:"to_format"`
	Content          string               `json:"content,omitempty"`
	Remote           *domain.RemoteInput  `json:"remote,omitempty"`
	ParseProcessors  []processorSpec      `json:"parse_processors,omitempty"`
	RenderProcessors []processorSpec      `json:"render_processors,omitempty"`
	Options          domain.RenderOptions `json:"options,omitempty"`
	Meta             map[string]string    `json:"meta,omitempty"`
}

type getFileInput struct {
	File    string            `json:"file"`
	Mode    string            `json:"mode,omitempty"`
	Target  string            `json:"target,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Refresh bool              `json:"refresh,omitempty"`
}

type putSubscriptionInput struct {
	subscription
}

type deleteSubscriptionInput struct {
	Name string `json:"name"`
}

type putFileInput struct {
	fileSpec
}

type deleteFileInput struct {
	Name string `json:"name"`
}
