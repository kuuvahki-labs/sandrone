package singbox

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderSingBoxSSPlugin(plugin string, options map[string]any) (string, string, error) {
	if isSingBoxSimpleObfsPlugin(plugin) {
		return "obfs-local", simpleObfsPluginOptionsRaw(options), nil
	}
	if strings.EqualFold(strings.TrimSpace(plugin), "v2ray-plugin") {
		normalized, err := singBoxV2RayPluginOptionsRaw(options)
		return "v2ray-plugin", normalized, err
	}
	return plugin, pluginOptionsRaw(options), nil
}

func singBoxV2RayPluginOptionsRaw(options map[string]any) (string, error) {
	if len(options) == 0 {
		return "", nil
	}
	if raw, ok := options["raw"]; ok {
		return fmt.Sprint(raw), nil
	}

	normalized := make(map[string]any, len(options))
	for key, value := range options {
		normalized[key] = value
	}
	if value, ok := normalized["skip-cert-verify"]; ok {
		enabled, valid := pluginBoolValue(value)
		if !valid {
			return "", domain.NewError(domain.CodeRenderFailed, "sing-box v2ray-plugin skip-cert-verify must be boolean")
		}
		delete(normalized, "skip-cert-verify")
		if enabled {
			return "", domain.NewError(domain.CodeRenderFailed, "sing-box v2ray-plugin cannot express skip-cert-verify")
		}
	}
	if value, ok := normalized["mux"]; ok {
		if enabled, valid := pluginBoolValue(value); valid {
			if enabled {
				normalized["mux"] = 1
			} else {
				normalized["mux"] = 0
			}
		}
	}
	if value, ok := normalized["tls"]; ok {
		if enabled, valid := pluginBoolValue(value); valid && !enabled {
			delete(normalized, "tls")
		}
	}
	return pluginOptionsRaw(normalized), nil
}

func pluginBoolValue(value any) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "true", "1", "yes", "y", "on":
		return true, true
	case "false", "0", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func isSingBoxSimpleObfsPlugin(plugin string) bool {
	switch strings.ToLower(strings.TrimSpace(plugin)) {
	case "obfs", "obfs-local", "simple-obfs":
		return true
	default:
		return false
	}
}

func simpleObfsPluginOptionsRaw(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	if raw, ok := options["raw"]; ok {
		parsed := parsePluginOptionsRaw(fmt.Sprint(raw))
		if len(parsed) == 0 {
			return fmt.Sprint(raw)
		}
		return simpleObfsPluginOptionsRaw(parsed)
	}
	mapped := map[string]any{}
	if mode, ok := firstNonEmptyPluginOption(options, "obfs", "mode"); ok {
		mapped["obfs"] = mode
	}
	if host, ok := firstNonEmptyPluginOption(options, "obfs-host", "host"); ok {
		mapped["obfs-host"] = host
	}
	return pluginOptionsRaw(mapped)
}

func parsePluginOptionsRaw(raw string) map[string]any {
	out := map[string]any{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func pluginOptionsRaw(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	if raw, ok := options["raw"]; ok {
		return fmt.Sprint(raw)
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, escapePluginOption(key)+"="+escapePluginOption(fmt.Sprint(options[key])))
	}
	return strings.Join(parts, ";")
}

func firstNonEmptyPluginOption(options map[string]any, keys ...string) (string, bool) {
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

func escapePluginOption(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `=`, `\=`)
	return replacer.Replace(value)
}
