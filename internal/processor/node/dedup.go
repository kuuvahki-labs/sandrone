package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// DedupParams chooses the dedup key. By default a node is identified by
// (type, server, port, uuid, password); set Strategy to "name" to dedup by
// name only.
type DedupParams struct {
	Strategy string   `json:"strategy,omitempty" jsonschema:"Key strategy used to identify duplicates" enum:"identity,name,fields" default:"identity"`
	Fields   []string `json:"fields,omitempty" jsonschema:"Node fields used when strategy is fields"`
}

type dedupProc struct {
	strategy string
	fields   []string
}

func buildDedup(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	var params DedupParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	strategy := strings.ToLower(strings.TrimSpace(params.Strategy))
	if strategy == "" {
		strategy = "identity"
	}
	switch strategy {
	case "identity", "name", "fields":
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("unknown dedup strategy %q", strategy),
			Processor: spec.Type,
		}
	}
	if strategy == "fields" && len(params.Fields) == 0 {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "dedup strategy=fields requires fields list",
			Processor: spec.Type,
		}
	}
	return &dedupProc{strategy: strategy, fields: params.Fields}, nil
}

func (p *dedupProc) Name() string { return "dedup" }

func (p *dedupProc) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	seen := map[string]struct{}{}
	out := make([]domain.NodeIR, 0, len(in.Nodes))
	for _, n := range in.Nodes {
		key := p.keyFor(n)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}

func (p *dedupProc) keyFor(n domain.NodeIR) string {
	switch p.strategy {
	case "name":
		return "name:" + n.Name
	case "fields":
		parts := make([]string, 0, len(p.fields))
		for _, f := range p.fields {
			parts = append(parts, fmt.Sprintf("%s=%s", f, fieldValueWide(n, f)))
		}
		return "fields:" + strings.Join(parts, "|")
	default:
		seed := fmt.Sprintf("%s|%s|%d|%s|%s", n.Type, n.Server, n.Port, n.UUID, n.Password)
		sum := sha256.Sum256([]byte(seed))
		return "identity:" + hex.EncodeToString(sum[:])
	}
}

func fieldValueWide(n domain.NodeIR, field string) string {
	switch strings.ToLower(field) {
	case "name":
		return n.Name
	case "type":
		return string(n.Type)
	case "server":
		return n.Server
	case "port":
		return fmt.Sprintf("%d", n.Port)
	case "uuid":
		return n.UUID
	case "password":
		return n.Password
	case "username":
		return n.Username
	case "cipher":
		return n.Cipher
	default:
		return ""
	}
}
