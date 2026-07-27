package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

type FileSpec struct {
	Name                  string            `json:"name" yaml:"name"`
	DisplayName           string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Kind                  FileKind          `json:"kind" yaml:"kind"`
	Source                FileSource        `json:"source" yaml:"source"`
	Config                *FileConfig       `json:"config,omitempty" yaml:"config,omitempty"`
	Processors            []ProcessorSpec   `json:"processors,omitempty" yaml:"processors,omitempty"`
	RenderCacheTTLSeconds *int              `json:"render_cache_ttl_seconds,omitempty" yaml:"render_cache_ttl_seconds,omitempty"`
	CreatedAt             time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt             time.Time         `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Meta                  map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type FileKind string

const (
	FileKindStatic       FileKind = "static"
	FileKindMihomo       FileKind = "mihomo"
	FileKindSingBox      FileKind = "sing-box"
	FileKindShadowrocket FileKind = "shadowrocket"
)

type FileConfig struct {
	Subscriptions []string        `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Settings      json.RawMessage `json:"settings,omitempty" yaml:"settings,omitempty"`
}

func (c *FileConfig) UnmarshalJSON(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	*c = FileConfig{}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := fields[name]
		switch name {
		case "subscriptions":
			if err := json.Unmarshal(raw, &c.Subscriptions); err != nil {
				return fmt.Errorf("config.subscriptions: %w", err)
			}
		case "settings":
			c.Settings = append(json.RawMessage(nil), raw...)
		default:
			return fmt.Errorf("config.%s: unknown field", name)
		}
	}
	return nil
}

func (c FileConfig) MarshalJSON() ([]byte, error) {
	type wire struct {
		Subscriptions []string        `json:"subscriptions,omitempty"`
		Settings      json.RawMessage `json:"settings,omitempty"`
	}
	return json.Marshal(wire(c))
}

func (c *FileConfig) UnmarshalYAML(node *yaml.Node) error {
	var value any
	if err := node.Decode(&value); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return c.UnmarshalJSON(body)
}

func (c FileConfig) MarshalYAML() (any, error) {
	settings := any(nil)
	if len(c.Settings) > 0 {
		if err := json.Unmarshal(c.Settings, &settings); err != nil {
			return nil, fmt.Errorf("config.settings: %w", err)
		}
	}
	wire := map[string]any{}
	if len(c.Subscriptions) > 0 {
		wire["subscriptions"] = c.Subscriptions
	}
	if len(c.Settings) > 0 {
		wire["settings"] = settings
	}
	return wire, nil
}

type FileAdaptiveGroupConfig struct {
	Type    string   `json:"type,omitempty" yaml:"type,omitempty"`
	Regions []string `json:"regions,omitempty" yaml:"regions,omitempty"`
}

type FileSource struct {
	Type    string       `json:"type" yaml:"type"`
	Content string       `json:"content,omitempty" yaml:"content,omitempty"`
	Remote  *RemoteInput `json:"remote,omitempty" yaml:"remote,omitempty"`
}

type NodeInput struct {
	Name            string            `json:"name" yaml:"name"`
	Type            string            `json:"type" yaml:"type"`
	Ref             ResourceRef       `json:"ref,omitempty" yaml:"ref,omitempty"`
	Format          string            `json:"format,omitempty" yaml:"format,omitempty"`
	Target          string            `json:"target,omitempty" yaml:"target,omitempty"`
	Nodes           []NodeIR          `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Content         string            `json:"content,omitempty" yaml:"content,omitempty"`
	Path            string            `json:"path,omitempty" yaml:"path,omitempty"`
	URL             string            `json:"url,omitempty" yaml:"url,omitempty"`
	UserAgent       string            `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	Proxy           string            `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	CacheTTLSeconds int               `json:"cache_ttl_seconds,omitempty" yaml:"cache_ttl_seconds,omitempty"`
	Required        bool              `json:"required,omitempty" yaml:"required,omitempty"`
	Meta            map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
}

type FileMergePolicy struct {
	Mode               string   `json:"mode,omitempty" yaml:"mode,omitempty"`
	Include            []string `json:"include,omitempty" yaml:"include,omitempty"`
	Separator          string   `json:"separator,omitempty" yaml:"separator,omitempty"`
	IgnoreFailedRemote bool     `json:"ignore_failed_remote,omitempty" yaml:"ignore_failed_remote,omitempty"`
}

type FileDocument struct {
	Name      string            `json:"name" yaml:"name"`
	Kind      string            `json:"kind" yaml:"kind"`
	MediaType string            `json:"media_type,omitempty" yaml:"media_type,omitempty"`
	Encoding  string            `json:"encoding,omitempty" yaml:"encoding,omitempty"`
	Parts     []FilePart        `json:"parts,omitempty" yaml:"parts,omitempty"`
	Content   []byte            `json:"content,omitempty" yaml:"content,omitempty"`
	Meta      map[string]string `json:"meta,omitempty" yaml:"meta,omitempty"`
	Warnings  []Warning         `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type FilePart struct {
	Name        string    `json:"name" yaml:"name"`
	Role        string    `json:"role" yaml:"role"`
	Kind        string    `json:"kind" yaml:"kind"`
	SourceRef   SourceRef `json:"source_ref,omitempty" yaml:"source_ref,omitempty"`
	Content     []byte    `json:"content,omitempty" yaml:"content,omitempty"`
	ContentHash string    `json:"content_hash,omitempty" yaml:"content_hash,omitempty"`
	Nodes       []NodeIR  `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

type Stage string

const (
	StageNodes Stage = "nodes"
	StageFile  Stage = "file"
)

type ProcessorSpec struct {
	Name   string                     `json:"name,omitempty" yaml:"name,omitempty"`
	Type   string                     `json:"type" yaml:"type"`
	Stage  Stage                      `json:"stage,omitempty" yaml:"stage,omitempty"`
	Params map[string]json.RawMessage `json:"params,omitempty" yaml:"params,omitempty"`
}

type FileProcessInput struct {
	Target  string       `json:"target" yaml:"target"`
	File    FileDocument `json:"file" yaml:"file"`
	Parts   []FilePart   `json:"parts,omitempty" yaml:"parts,omitempty"`
	Request RequestInfo  `json:"request,omitempty" yaml:"request,omitempty"`
}

type FileProcessOutput struct {
	File     FileDocument `json:"file" yaml:"file"`
	Warnings []Warning    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type ScriptProduceOptions struct {
	Target string            `json:"target,omitempty" yaml:"target,omitempty"`
	Args   map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
}

type ScriptSubscriptionProduceResult struct {
	Kind    string   `json:"kind" yaml:"kind"`
	Nodes   []NodeIR `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Target  string   `json:"target,omitempty" yaml:"target,omitempty"`
	Content string   `json:"content,omitempty" yaml:"content,omitempty"`
	Report  Report   `json:"report,omitempty" yaml:"report,omitempty"`
}
