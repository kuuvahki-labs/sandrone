package filedriver

import (
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func singBoxOutboundsWithGroups(groups []map[string]any, names []string, nodeOutbounds []any) []any {
	out := expandConfigGroups(groups, names)
	out = append(out, map[string]any{"type": "direct", "tag": "direct"}, map[string]any{"type": "block", "tag": "block"})
	out = append(out, nodeOutbounds...)
	return out
}

func normalizeConfigMap(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeConfigMap(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeConfigMap(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, value := range typed {
			out[index] = normalizeConfigMap(value)
		}
		return out
	default:
		return value
	}
}

func configMapList(values []map[string]any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		normalized, ok := normalizeConfigMap(value).(map[string]any)
		if ok {
			out = append(out, normalized)
		}
	}
	return out
}

func expandConfigGroups(groups []map[string]any, names []string) []any {
	out := make([]any, 0, len(groups))
	for _, group := range groups {
		normalized, ok := normalizeConfigMap(group).(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"proxies", "outbounds"} {
			if list, ok := normalized[field].([]any); ok {
				normalized[field] = expandNodeRefs(list, names)
			}
		}
		out = append(out, normalized)
	}
	return out
}

func expandNodeRefs(values []any, names []string) []any {
	out := make([]any, 0, len(values)+len(names))
	for _, value := range values {
		if text, ok := value.(string); ok && text == "$nodes" {
			for _, name := range names {
				out = append(out, name)
			}
			continue
		}
		out = append(out, value)
	}
	return out
}

func mihomoRuleProvidersFromConfig(ruleSets []map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, ruleSet := range ruleSets {
		normalized, ok := normalizeConfigMap(ruleSet).(map[string]any)
		if !ok {
			continue
		}
		name, ok := normalized["name"].(string)
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "mihomo rule set entries require a name")
		}
		delete(normalized, "name")
		out[name] = normalized
	}
	return out, nil
}
