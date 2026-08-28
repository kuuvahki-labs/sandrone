// Package script implements the JavaScript script processor. It uses goja as a
// pure-Go ECMAScript runtime and exposes a tightly scoped api object instead of
// Node.js globals.
package script

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// ScriptEnvelope is the JSON-shaped payload passed to scripts. It is
// intentionally distinct from the in-process NodeIR / FileDocument structs
// so that scripts can mutate fields freely while we still validate the
// result before merging it back.
type ScriptEnvelope struct {
	Version       int                  `json:"version"`
	Stage         string               `json:"stage"`
	Target        string               `json:"target,omitempty"`
	RenderOptions domain.RenderOptions `json:"render_options"`

	Context  domain.NodeContext  `json:"context,omitempty"`
	Request  domain.RequestInfo  `json:"request"`
	Response domain.ResponseInfo `json:"response"`
	Args     map[string]any      `json:"args,omitempty"`

	Nodes []ScriptNode `json:"nodes,omitempty"`
	File  *ScriptFile  `json:"file,omitempty"`
	Parts []ScriptPart `json:"parts,omitempty"`

	Warnings []domain.Warning `json:"warnings,omitempty"`
}

// ScriptNode mirrors NodeIR with map[string]any for the structured options.
// `Ext` is a free-form bag for script-added fields that don't map back to
// known IR fields.
type ScriptNode struct {
	runtimeID      string
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Server         string            `json:"server,omitempty"`
	Port           uint16            `json:"port,omitempty"`
	Network        string            `json:"network,omitempty"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	UUID           string            `json:"uuid,omitempty"`
	Cipher         string            `json:"cipher,omitempty"`
	AlterID        int               `json:"alter_id,omitempty"`
	Flow           string            `json:"flow,omitempty"`
	Encryption     string            `json:"encryption,omitempty"`
	Token          string            `json:"token,omitempty"`
	PacketEncoding string            `json:"packet_encoding,omitempty"`
	Plugin         string            `json:"plugin,omitempty"`
	PluginOptions  map[string]any    `json:"plugin_options,omitempty"`
	ShadowsocksR   map[string]any    `json:"shadowsocksr,omitempty"`
	Snell          map[string]any    `json:"snell,omitempty"`
	AnyTLS         map[string]any    `json:"anytls,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Path           string            `json:"path,omitempty"`
	TLS            map[string]any    `json:"tls,omitempty"`
	Dialer         map[string]any    `json:"dialer,omitempty"`
	Transport      map[string]any    `json:"transport,omitempty"`
	Multiplex      map[string]any    `json:"multiplex,omitempty"`
	UDPOverTCP     map[string]any    `json:"udp_over_tcp,omitempty"`
	Hysteria       map[string]any    `json:"hysteria,omitempty"`
	TUIC           map[string]any    `json:"tuic,omitempty"`
	Mieru          map[string]any    `json:"mieru,omitempty"`
	WireGuard      map[string]any    `json:"wireguard,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Meta           map[string]string `json:"meta,omitempty"`
	Raw            map[string]any    `json:"raw,omitempty"`
	Ext            map[string]any    `json:"ext,omitempty"`
	Lossy          bool              `json:"lossy,omitempty"`
	Warnings       []domain.Warning  `json:"warnings,omitempty"`
	SourceFormat   string            `json:"source_format,omitempty"`
}

type ScriptFile struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	MediaType string            `json:"media_type,omitempty"`
	Encoding  string            `json:"encoding,omitempty"`
	Content   string            `json:"content"`
	Meta      map[string]string `json:"meta,omitempty"`
	Warnings  []domain.Warning  `json:"warnings,omitempty"`
}

type ScriptPart struct {
	Name        string           `json:"name"`
	Role        string           `json:"role"`
	Kind        string           `json:"kind"`
	SourceRef   domain.SourceRef `json:"source_ref,omitempty"`
	Content     string           `json:"content"`
	ContentHash string           `json:"content_hash,omitempty"`
	Nodes       []ScriptNode     `json:"nodes,omitempty"`
}

// nodeToScript converts a NodeIR to its ScriptNode shape via JSON round-trip.
// Unknown top-level fields are preserved in Ext so a script that adds fields
// can still be re-encoded back into an IR.
func nodeToScript(n domain.NodeIR) (ScriptNode, error) {
	body, err := json.Marshal(n) //nolint:gosec // NodeIR contains proxy credentials by design; this marshals in memory for script envelope conversion.
	if err != nil {
		return ScriptNode{}, err
	}
	var s ScriptNode
	if err := json.Unmarshal(body, &s); err != nil {
		return ScriptNode{}, err
	}
	s.runtimeID = domain.NodeRuntimeID(n)
	return s, nil
}

func scriptToNode(s ScriptNode) (domain.NodeIR, []domain.Warning, error) {
	body, err := json.Marshal(s) //nolint:gosec // ScriptNode may contain proxy credentials by design; this marshals in memory back into NodeIR.
	if err != nil {
		return domain.NodeIR{}, nil, err
	}
	var node domain.NodeIR
	if err := json.Unmarshal(body, &node); err != nil {
		return domain.NodeIR{}, nil, err
	}
	domain.SetNodeRuntimeID(&node, s.runtimeID)
	warnings := []domain.Warning{}
	if node.Type == "" {
		return node, nil, fmt.Errorf("script returned node %q without type", node.Name)
	}
	if node.Name == "" {
		return node, nil, fmt.Errorf("script returned node without name")
	}
	if s.Ext != nil {
		if node.Raw == nil {
			node.Raw = map[string]json.RawMessage{}
		}
		for k, v := range s.Ext {
			raw, err := json.Marshal(v)
			if err != nil {
				return node, nil, fmt.Errorf("encode ext field %s: %w", k, err)
			}
			node.Raw["script.ext."+k] = raw
			warnings = append(warnings, domain.Warning{
				Code:    "script_ext_field",
				Message: fmt.Sprintf("script added ext field %q to node %q", k, node.Name),
				Node:    node.Name,
				Field:   k,
			})
		}
	}
	return node, warnings, nil
}

func nodesToScript(nodes []domain.NodeIR) ([]ScriptNode, error) {
	out := make([]ScriptNode, len(nodes))
	for i, n := range nodes {
		s, err := nodeToScript(n)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

func scriptToNodes(nodes []ScriptNode) ([]domain.NodeIR, []domain.Warning, error) {
	out := make([]domain.NodeIR, 0, len(nodes))
	warnings := []domain.Warning{}
	for i, s := range nodes {
		node, w, err := scriptToNode(s)
		if err != nil {
			return nil, nil, fmt.Errorf("node %d: %w", i, err)
		}
		out = append(out, node)
		warnings = append(warnings, w...)
	}
	return out, warnings, nil
}

func fileToScript(doc domain.FileDocument) *ScriptFile {
	return &ScriptFile{
		Name:      doc.Name,
		Kind:      doc.Kind,
		MediaType: doc.MediaType,
		Encoding:  doc.Encoding,
		Content:   string(doc.Content),
		Meta:      maps.Clone(doc.Meta),
		Warnings:  append([]domain.Warning{}, doc.Warnings...),
	}
}

func partsToScript(parts []domain.FilePart) ([]ScriptPart, error) {
	out := make([]ScriptPart, len(parts))
	for i, p := range parts {
		nodes, err := nodesToScript(p.Nodes)
		if err != nil {
			return nil, err
		}
		out[i] = ScriptPart{
			Name:        p.Name,
			Role:        p.Role,
			Kind:        p.Kind,
			SourceRef:   p.SourceRef,
			Content:     string(p.Content),
			ContentHash: p.ContentHash,
			Nodes:       nodes,
		}
	}
	return out, nil
}

func scriptToFile(s *ScriptFile, base domain.FileDocument) domain.FileDocument {
	if s == nil {
		return base
	}
	out := base
	out.Name = s.Name
	out.Kind = s.Kind
	if s.MediaType != "" {
		out.MediaType = s.MediaType
	}
	if s.Encoding != "" {
		out.Encoding = s.Encoding
	}
	out.Content = []byte(s.Content)
	if s.Meta != nil {
		out.Meta = maps.Clone(s.Meta)
	}
	if s.Warnings != nil {
		out.Warnings = append([]domain.Warning{}, s.Warnings...)
	}
	return out
}
