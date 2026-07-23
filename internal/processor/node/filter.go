// Package node contains built-in node-stage processors. They are pure
// transforms over []domain.NodeIR and never reach into store, network or other
// side effects.
package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// FilterParams selects or drops nodes using one explicit match rule.
type FilterParams struct {
	Action  string   `json:"action" jsonschema:"Whether matching nodes are kept or dropped" enum:"keep,drop"`
	Field   string   `json:"field" jsonschema:"Node field to inspect" enum:"name,type,server"`
	Match   string   `json:"match" jsonschema:"Matching strategy" enum:"regex,in"`
	Pattern string   `json:"pattern,omitempty" jsonschema:"Regular expression used when match is regex"`
	Values  []string `json:"values,omitempty" jsonschema:"Accepted values used when match is in"`
}

type filterProc struct {
	action string
	field  string
	match  string
	re     *regexp.Regexp
	values map[string]struct{}
}

func buildFilter(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	var params FilterParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	action := strings.ToLower(strings.TrimSpace(params.Action))
	switch action {
	case "keep", "drop":
	case "":
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "filter action is required",
			Processor: spec.Type,
		}
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("filter action %q is not supported", action),
			Processor: spec.Type,
		}
	}
	field := strings.ToLower(strings.TrimSpace(params.Field))
	if !isFilterFieldSupported(field) {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("filter field %q is not supported", field),
			Processor: spec.Type,
		}
	}
	match := strings.ToLower(strings.TrimSpace(params.Match))
	p := &filterProc{action: action, field: field, match: match}
	switch match {
	case "regex":
		if params.Pattern == "" {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "filter match=regex requires pattern",
				Processor: spec.Type,
			}
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("compile filter pattern %q", params.Pattern),
				Processor: spec.Type,
				Cause:     err,
			}
		}
		p.re = re
	case "in":
		values := normalizeFilterValues(field, params.Values)
		if len(values) == 0 {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "filter match=in requires values",
				Processor: spec.Type,
			}
		}
		p.values = values
	case "":
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "filter match is required",
			Processor: spec.Type,
		}
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("filter match %q is not supported", match),
			Processor: spec.Type,
		}
	}
	return p, nil
}

func (p *filterProc) Name() string { return "filter" }

func (p *filterProc) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := make([]domain.NodeIR, 0, len(in.Nodes))
	for _, n := range in.Nodes {
		matched := p.matches(n)
		if (p.action == "keep" && matched) || (p.action == "drop" && !matched) {
			out = append(out, n)
		}
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}

func (p *filterProc) matches(n domain.NodeIR) bool {
	value := fieldValue(n, p.field)
	switch p.match {
	case "regex":
		return p.re.MatchString(value)
	case "in":
		_, ok := p.values[normalizeFilterValue(p.field, value)]
		return ok
	default:
		return false
	}
}

func normalizeFilterValues(field string, values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeFilterValue(field, value)
		if normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func normalizeFilterValue(field, value string) string {
	value = strings.TrimSpace(value)
	if field == "type" {
		return strings.ToLower(value)
	}
	return value
}

// fieldValue returns the string view of a NodeIR field. Supported fields are
// kept intentionally narrow to fields that are reasonable to regex against.
func fieldValue(n domain.NodeIR, field string) string {
	switch field {
	case "name":
		return n.Name
	case "type":
		return string(n.Type)
	case "server":
		return n.Server
	default:
		return ""
	}
}

func isFilterFieldSupported(field string) bool {
	switch field {
	case "name", "type", "server":
		return true
	default:
		return false
	}
}
