package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// RenameParams rewrites each node name once. Cleanup always runs before the
// optional rename mode; use multiple rename processors for multiple passes.
type RenameParams struct {
	Trim        bool     `json:"trim,omitempty" jsonschema:"Trim leading and trailing whitespace"`
	Strip       []string `json:"strip,omitempty" jsonschema:"Literal fragments removed from every node name"`
	Mode        string   `json:"mode,omitempty" jsonschema:"Optional name rewrite mode" enum:"replace,prefix,suffix,template"`
	Pattern     string   `json:"pattern,omitempty" jsonschema:"Regular expression used by replace mode"`
	Replacement string   `json:"replacement,omitempty" jsonschema:"Replacement used by replace mode"`
	Value       string   `json:"value,omitempty" jsonschema:"Value used by prefix suffix or template mode"`
}

type renameProc struct {
	trim        bool
	strip       []string
	mode        string
	pattern     *regexp.Regexp
	replacement string
	value       string
}

func buildRename(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	var params RenameParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	strip := nonEmptyStrings(params.Strip)
	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if !params.Trim && len(strip) == 0 && mode == "" {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "rename requires trim, strip, or mode",
			Processor: spec.Type,
		}
	}
	rename := &renameProc{
		trim:        params.Trim,
		strip:       strip,
		mode:        mode,
		replacement: params.Replacement,
		value:       params.Value,
	}
	switch mode {
	case "":
		return rename, nil
	case "replace":
		if params.Pattern == "" {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "rename replace requires pattern",
				Processor: spec.Type,
			}
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("compile rename pattern %q", params.Pattern),
				Processor: spec.Type,
				Cause:     err,
			}
		}
		rename.pattern = re
		return rename, nil
	case "prefix", "suffix", "template":
		if params.Value == "" {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("rename %s requires value", mode),
				Processor: spec.Type,
			}
		}
		return rename, nil
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("unknown rename mode %q", mode),
			Processor: spec.Type,
		}
	}
}

func (p *renameProc) Name() string { return "rename" }

func (p *renameProc) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := make([]domain.NodeIR, len(in.Nodes))
	for i, n := range in.Nodes {
		for _, fragment := range p.strip {
			n.Name = strings.ReplaceAll(n.Name, fragment, "")
		}
		if p.trim {
			n.Name = strings.TrimSpace(n.Name)
		}
		switch p.mode {
		case "replace":
			n.Name = p.pattern.ReplaceAllString(n.Name, p.replacement)
		case "prefix":
			n.Name = p.value + n.Name
		case "suffix":
			n.Name += p.value
		case "template":
			n.Name = applyTemplate(p.value, n)
		}
		out[i] = n
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func applyTemplate(tpl string, n domain.NodeIR) string {
	r := strings.NewReplacer(
		"{name}", n.Name,
		"{type}", string(n.Type),
		"{server}", n.Server,
		"{source_format}", n.SourceFormat,
	)
	return r.Replace(tpl)
}
