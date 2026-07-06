package mihomo

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func unknownWarnings(node domain.NodeIR, raw map[string]json.RawMessage, source string, nodeIndex int, nodeContext domain.WarningNodeContext) []domain.Warning {
	index := nodeIndex
	return shared.ParseUnknownWarningsWithContext(node, raw, source, &index, &nodeContext)
}

func mihomoWarningNodeContext(node domain.NodeIR, proxy map[string]any) domain.WarningNodeContext {
	return domain.WarningNodeContext{
		Format: "mihomo",
		Name:   node.Name,
		Type:   node.Type,
		Server: node.Server,
		Port:   node.Port,
		Raw:    proxy,
	}
}

func mapStringListToString(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		values := shared.StringSliceValue(value)
		if len(values) > 0 {
			out[key] = values[0]
		} else {
			out[key] = shared.StringValue(value)
		}
	}
	return out
}

func intString(v any) string {
	n, err := shared.IntValue(v)
	if err != nil || n == 0 {
		return shared.StringValue(v)
	}
	return fmt.Sprint(n)
}

func intValueZero(v any) int {
	n, err := shared.IntValue(v)
	if err != nil {
		return 0
	}
	return n
}

func uint8SliceValue(v any) []uint8 {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]uint8, 0, len(items))
	for _, item := range items {
		n, err := shared.IntValue(item)
		if err == nil && n >= 0 && n <= 255 {
			out = append(out, uint8(n))
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(values ...any) []string {
	for _, value := range values {
		items := shared.StringSliceValue(value)
		if len(items) > 0 {
			return items
		}
	}
	return nil
}
