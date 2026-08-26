package filedriver

import (
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderedYAMLList(body []byte, key string) ([]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "parse rendered mihomo proxies", err)
	}
	return anyList(doc[key]), nil
}

func namesFromMihomoProxies(proxies []any) []string {
	names := []string{}
	for _, item := range proxies {
		proxy, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := proxy["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return uniqueStrings(names)
}

func namesFromSingBoxOutbounds(outbounds []any, endpoints []any) []string {
	names := []string{}
	for _, item := range append(outbounds, endpoints...) {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := outbound["tag"].(string); ok && tag != "" {
			names = append(names, tag)
		}
	}
	return uniqueStrings(names)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func anyList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{}
}

func mapValue(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}
