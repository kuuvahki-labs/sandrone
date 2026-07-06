package service

import (
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const configProbeURL = "http://www.gstatic.com/generate_204"

func mihomoGroups(preset string, names []string) []any {
	switch preset {
	case "minimal":
		return []any{map[string]any{
			"name":    "Proxy",
			"type":    "select",
			"proxies": append(append([]string{}, names...), "DIRECT"),
		}}
	case "region":
		return regionMihomoGroups(names)
	default:
		return []any{
			map[string]any{
				"name":    "Proxy",
				"type":    "select",
				"proxies": append([]string{"Auto", "Fallback"}, append(append([]string{}, names...), "DIRECT")...),
			},
			map[string]any{
				"name":     "Auto",
				"type":     "url-test",
				"proxies":  append([]string{}, names...),
				"url":      configProbeURL,
				"interval": 300,
			},
			map[string]any{
				"name":     "Fallback",
				"type":     "fallback",
				"proxies":  append([]string{}, names...),
				"url":      configProbeURL,
				"interval": 300,
			},
		}
	}
}

func regionMihomoGroups(names []string) []any {
	regions := regionBuckets(names)
	groups := make([]any, 0, 8)
	groups = append(groups, map[string]any{
		"name":    "Proxy",
		"type":    "select",
		"proxies": append([]string{"Auto", "Hong Kong", "Taiwan", "Japan", "Singapore", "United States", "Other"}, append(append([]string{}, names...), "DIRECT")...),
	})
	for _, region := range []string{"Hong Kong", "Taiwan", "Japan", "Singapore", "United States", "Other"} {
		proxies := regions[region]
		if len(proxies) == 0 {
			proxies = append([]string{}, names...)
		}
		groups = append(groups, map[string]any{"name": region, "type": "select", "proxies": proxies})
	}
	groups = append(groups, map[string]any{"name": "Auto", "type": "url-test", "proxies": append([]string{}, names...), "url": configProbeURL, "interval": 300})
	return groups
}

func mihomoRuleProviders(string) map[string]any {
	return map[string]any{
		"private": map[string]any{
			"type":     "inline",
			"behavior": "classical",
			"payload": []string{
				"DOMAIN-SUFFIX,local",
				"IP-CIDR,10.0.0.0/8,no-resolve",
				"IP-CIDR,172.16.0.0/12,no-resolve",
				"IP-CIDR,192.168.0.0/16,no-resolve",
			},
		},
		"reject": map[string]any{
			"type":     "inline",
			"behavior": "classical",
			"payload":  []string{"DOMAIN-SUFFIX,invalid"},
		},
	}
}

func mihomoRules() []any {
	return []any{
		"RULE-SET,private,DIRECT",
		"RULE-SET,reject,REJECT",
		"GEOIP,CN,DIRECT",
		"MATCH,Proxy",
	}
}

func singBoxOutbounds(preset string, names []string, nodeOutbounds []any) []any {
	out := []any{}
	switch preset {
	case "minimal":
		out = append(out, map[string]any{
			"type":      "selector",
			"tag":       "Proxy",
			"outbounds": append(append([]string{}, names...), "direct"),
		})
	case "region":
		out = append(out, regionSingBoxOutbounds(names)...)
	default:
		out = append(out,
			map[string]any{
				"type":      "selector",
				"tag":       "Proxy",
				"outbounds": append([]string{"Auto", "Fallback"}, append(append([]string{}, names...), "direct")...),
				"default":   "Auto",
			},
			map[string]any{
				"type":      "urltest",
				"tag":       "Auto",
				"outbounds": append([]string{}, names...),
				"url":       configProbeURL,
				"interval":  "5m",
			},
			map[string]any{
				"type":      "selector",
				"tag":       "Fallback",
				"outbounds": append([]string{}, names...),
			},
		)
	}
	out = append(out, map[string]any{"type": "direct", "tag": "direct"}, map[string]any{"type": "block", "tag": "block"})
	out = append(out, nodeOutbounds...)
	return out
}

func singBoxOutboundsWithGroups(groups []map[string]any, names []string, nodeOutbounds []any) []any {
	out := expandConfigGroups(groups, names)
	out = append(out, map[string]any{"type": "direct", "tag": "direct"}, map[string]any{"type": "block", "tag": "block"})
	out = append(out, nodeOutbounds...)
	return out
}

func regionSingBoxOutbounds(names []string) []any {
	regions := regionBuckets(names)
	out := make([]any, 0, 8)
	out = append(out, map[string]any{
		"type":      "selector",
		"tag":       "Proxy",
		"outbounds": append([]string{"Auto", "Hong Kong", "Taiwan", "Japan", "Singapore", "United States", "Other"}, append(append([]string{}, names...), "direct")...),
		"default":   "Auto",
	})
	for _, region := range []string{"Hong Kong", "Taiwan", "Japan", "Singapore", "United States", "Other"} {
		proxies := regions[region]
		if len(proxies) == 0 {
			proxies = append([]string{}, names...)
		}
		out = append(out, map[string]any{"type": "selector", "tag": region, "outbounds": proxies})
	}
	out = append(out, map[string]any{"type": "urltest", "tag": "Auto", "outbounds": append([]string{}, names...), "url": configProbeURL, "interval": "5m"})
	return out
}

func singBoxRuleSets(string) []any {
	return []any{
		map[string]any{
			"type": "inline",
			"tag":  "private",
			"rules": []any{
				map[string]any{"domain_suffix": []string{"local"}},
				map[string]any{"ip_cidr": []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}},
			},
		},
		map[string]any{
			"type":  "inline",
			"tag":   "reject",
			"rules": []any{map[string]any{"domain_suffix": []string{"invalid"}}},
		},
	}
}

func singBoxRules() []any {
	return []any{
		map[string]any{"rule_set": []string{"private"}, "outbound": "direct"},
		map[string]any{"rule_set": []string{"reject"}, "outbound": "block"},
		map[string]any{"ip_is_private": true, "outbound": "direct"},
		map[string]any{"outbound": "Proxy"},
	}
}

func regionBuckets(names []string) map[string][]string {
	regions := map[string][]string{
		"Hong Kong":     {},
		"Taiwan":        {},
		"Japan":         {},
		"Singapore":     {},
		"United States": {},
		"Other":         {},
	}
	for _, name := range names {
		region := regionName(name)
		regions[region] = append(regions[region], name)
	}
	return regions
}

func regionName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "hk") || strings.Contains(name, "香港"):
		return "Hong Kong"
	case strings.Contains(lower, "tw") || strings.Contains(name, "台湾") || strings.Contains(name, "臺灣"):
		return "Taiwan"
	case strings.Contains(lower, "jp") || strings.Contains(name, "日本"):
		return "Japan"
	case strings.Contains(lower, "sg") || strings.Contains(name, "新加坡"):
		return "Singapore"
	case strings.Contains(lower, "us") || strings.Contains(name, "美国") || strings.Contains(name, "美國"):
		return "United States"
	default:
		return "Other"
	}
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
