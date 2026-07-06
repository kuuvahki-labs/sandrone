package singbox

import (
	"fmt"
	"sort"
	"strings"
)

func renderSingBoxSSPlugin(plugin string, options map[string]any) (string, string) {
	if isSingBoxSimpleObfsPlugin(plugin) {
		return "obfs-local", simpleObfsPluginOptionsRaw(options)
	}
	return plugin, pluginOptionsRaw(options)
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
