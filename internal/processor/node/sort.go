package node

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// SortParams sorts nodes stably. By is a comma-separated list of fields with
// optional + / - prefix to denote ascending / descending order. Default is
// "+name".
type SortParams struct {
	By string `json:"by,omitempty"`
}

type sortKey struct {
	field string
	desc  bool
}

type sortProc struct {
	keys []sortKey
}

func buildSort(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	var params SortParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(params.By)
	if raw == "" {
		raw = "+name"
	}
	parts := strings.Split(raw, ",")
	keys := make([]sortKey, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		k := sortKey{}
		switch key[0] {
		case '-':
			k.desc = true
			key = key[1:]
		case '+':
			key = key[1:]
		}
		k.field = strings.ToLower(strings.TrimSpace(key))
		if k.field == "" {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("invalid sort key %q", part),
				Processor: spec.Type,
			}
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "sort requires at least one key",
			Processor: spec.Type,
		}
	}
	return &sortProc{keys: keys}, nil
}

func (p *sortProc) Name() string { return "sort" }

func (p *sortProc) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := make([]domain.NodeIR, len(in.Nodes))
	copy(out, in.Nodes)
	sort.SliceStable(out, func(i, j int) bool {
		for _, k := range p.keys {
			a := fieldValueWide(out[i], k.field)
			b := fieldValueWide(out[j], k.field)
			if a == b {
				continue
			}
			if k.desc {
				return a > b
			}
			return a < b
		}
		return false
	})
	return domain.NodeProcessOutput{Nodes: out}, nil
}
