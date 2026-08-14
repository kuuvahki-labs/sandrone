package node

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// DedupParams chooses how duplicate nodes or names are handled.
type DedupParams struct {
	Strategy string   `json:"strategy,omitempty" jsonschema:"Strategy used to handle duplicates" enum:"identity,name,fields,random_suffix" default:"name"`
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
		strategy = "name"
	}
	switch strategy {
	case "identity", "name", "fields", "random_suffix":
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
	if p.strategy == "random_suffix" {
		return p.applyRandomSuffix(in)
	}
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

func (p *dedupProc) applyRandomSuffix(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	usedNames := make(map[string]struct{}, len(in.Nodes))
	for _, node := range in.Nodes {
		usedNames[node.Name] = struct{}{}
	}
	seenNames := make(map[string]struct{}, len(in.Nodes))
	out := make([]domain.NodeIR, 0, len(in.Nodes))
	for _, node := range in.Nodes {
		if _, exists := seenNames[node.Name]; !exists {
			seenNames[node.Name] = struct{}{}
			out = append(out, node)
			continue
		}
		name, err := uniqueRandomName(node.Name, usedNames)
		if err != nil {
			return domain.NodeProcessOutput{}, err
		}
		node.Name = name
		usedNames[name] = struct{}{}
		out = append(out, node)
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}

func uniqueRandomName(base string, used map[string]struct{}) (string, error) {
	randomStart, err := cryptorand.Int(cryptorand.Reader, big.NewInt(10_000))
	if err != nil {
		return "", fmt.Errorf("generate random suffix for node name %q: %w", base, err)
	}
	start := int(randomStart.Int64())
	for offset := range 10_000 {
		digits := (start + offset) % 10_000
		candidate := fmt.Sprintf("%s-%04d", base, digits)
		if base == "" {
			candidate = fmt.Sprintf("%04d", digits)
		}
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no four-digit random suffix is available for node name %q", base)
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
