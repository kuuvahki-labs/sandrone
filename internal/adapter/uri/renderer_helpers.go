package uri

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func hostPort(host string, port uint16) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

func hostOnly(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func hysteria2URIHost(node domain.NodeIR) string {
	portSpec := strconv.Itoa(int(node.Port))
	if node.Hysteria != nil && len(node.Hysteria.ServerPorts) > 0 {
		portSpec = strings.Join(node.Hysteria.ServerPorts, ",")
	}
	return hostPortWithPortSpec(node.Server, portSpec)
}

func hostPortWithPortSpec(host, portSpec string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + portSpec
	}
	return host + ":" + portSpec
}

func renderSIP002Plugin(plugin string, options map[string]any) string {
	if plugin == "" || strings.Contains(plugin, ";") || len(options) == 0 {
		return plugin
	}
	if raw, ok := options["raw"]; ok && fmt.Sprint(raw) != "" {
		return plugin + ";" + fmt.Sprint(raw)
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	parts = append(parts, plugin)
	for _, key := range keys {
		parts = append(parts, escapePluginOption(key)+"="+escapePluginOption(fmt.Sprint(options[key])))
	}
	return strings.Join(parts, ";")
}

func escapePluginOption(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `:`, `\:`, `=`, `\=`)
	return replacer.Replace(value)
}

func withFragment(raw, name string) string {
	if name == "" {
		return raw
	}
	return raw + "#" + url.QueryEscape(name)
}

func firstNonEmptyURI(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mieruProfileName(name, portSpec, protocol string) string {
	suffix := ":" + portSpec + "/" + protocol
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return name
}
