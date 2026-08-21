package mihomo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderMihomoSSPlugin(node domain.NodeIR) (string, map[string]any, error) {
	plugin := node.Plugin
	options := node.PluginOptions
	outOptions := options
	if isSimpleObfsPlugin(plugin) {
		if normalized := mihomoSimpleObfsOptions(options); len(normalized) > 0 {
			return "obfs", normalized, nil
		}
		return "obfs", outOptions, nil
	}
	if strings.EqualFold(strings.TrimSpace(plugin), "v2ray-plugin") {
		normalized, issues := mihomoV2RayPluginOptions(options)
		if len(issues) > 0 {
			return "", nil, domain.NewError(
				domain.CodeRenderFailed,
				"mihomo v2ray-plugin options contain unsupported, invalid, or conflicting connection parameters: "+strings.Join(issues, ", "),
			)
		}
		return "v2ray-plugin", normalized, nil
	}
	return plugin, outOptions, nil
}

func isSimpleObfsPlugin(plugin string) bool {
	switch strings.ToLower(strings.TrimSpace(plugin)) {
	case "obfs", "obfs-local", "simple-obfs":
		return true
	default:
		return false
	}
}

func mihomoSimpleObfsOptions(options map[string]any) map[string]any {
	if len(options) == 0 {
		return nil
	}
	if rawValue, ok := options["raw"]; ok {
		return parseSIP002SimpleObfsOptions(fmt.Sprint(rawValue))
	}
	out := map[string]any{}
	if mode, ok := firstNonEmptyOptionString(options, "mode", "obfs"); ok {
		out["mode"] = mode
	}
	if host, ok := firstNonEmptyOptionString(options, "host", "obfs-host"); ok {
		out["host"] = host
	}
	return out
}

func parseSIP002SimpleObfsOptions(raw string) map[string]any {
	out := map[string]any{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "obfs", "mode":
			if value != "" {
				out["mode"] = value
			}
		case "obfs-host", "host":
			if value != "" {
				out["host"] = value
			}
		}
	}
	return out
}

type sip002PluginOption struct {
	key      string
	value    string
	hasValue bool
}

func mihomoV2RayPluginOptions(options map[string]any) (map[string]any, []string) {
	if len(options) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	issues := map[string]bool{}
	if rawValue, ok := options["raw"]; ok {
		for _, option := range parseSIP002PluginOptions(fmt.Sprint(rawValue)) {
			if !applyMihomoV2RayPluginOption(out, option.key, option.value, option.hasValue, true) {
				issues[option.key] = true
			}
		}
	}
	for _, key := range sortedMapKeys(options) {
		if key == "raw" {
			continue
		}
		value := options[key]
		if !applyMihomoV2RayPluginOption(out, key, fmt.Sprint(value), true, true) {
			issues[key] = true
		}
	}
	issueKeys := make([]string, 0, len(issues))
	for key := range issues {
		issueKeys = append(issueKeys, key)
	}
	sort.Strings(issueKeys)
	return out, issueKeys
}

func applyMihomoV2RayPluginOption(out map[string]any, rawKey, value string, hasValue, rejectConflict bool) bool {
	key := strings.ToLower(strings.TrimSpace(rawKey))
	value = strings.TrimSpace(value)
	var normalized any
	switch key {
	case "mode", "host", "path":
		if !hasValue || value == "" {
			return false
		}
		normalized = value
	case "tls", "mux", "skip-cert-verify":
		if !hasValue || value == "" {
			normalized = true
			break
		}
		switch strings.ToLower(value) {
		case "true", "1", "yes", "y", "on":
			normalized = true
		case "false", "0", "no", "n", "off":
			normalized = false
		default:
			return false
		}
	default:
		return false
	}
	if existing, ok := out[key]; ok && rejectConflict && fmt.Sprint(existing) != fmt.Sprint(normalized) {
		return false
	}
	out[key] = normalized
	return true
}

func parseSIP002PluginOptions(raw string) []sip002PluginOption {
	parts := splitSIP002PluginOption(raw, ';')
	out := make([]sip002PluginOption, 0, len(parts))
	for _, part := range parts {
		keyValue := splitSIP002PluginOption(part, '=')
		if len(keyValue) == 0 || strings.TrimSpace(keyValue[0]) == "" {
			continue
		}
		option := sip002PluginOption{key: keyValue[0]}
		if len(keyValue) > 1 {
			option.value = strings.Join(keyValue[1:], "=")
			option.hasValue = true
		}
		out = append(out, option)
	}
	return out
}

func splitSIP002PluginOption(raw string, delimiter byte) []string {
	parts := []string{}
	var current strings.Builder
	for i := 0; i < len(raw); i++ {
		char := raw[i]
		if char == '\\' && i+1 < len(raw) {
			next := raw[i+1]
			if next == delimiter || next == '\\' {
				current.WriteByte(next)
				i++
				continue
			}
		}
		if char == delimiter {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(char)
	}
	parts = append(parts, current.String())
	return parts
}

func firstNonEmptyOptionString(options map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := options[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text, true
		}
	}
	return "", false
}
