package mihomo

import (
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func skippedMihomoProxyWarning(nodeIndex int, item any, proxy map[string]any, err error) domain.Warning {
	index := nodeIndex
	context := skippedMihomoProxyContext(item, proxy)
	return domain.Warning{
		Code:        "parse_proxy_skipped",
		Message:     err.Error(),
		Source:      "mihomo",
		NodeIndex:   &index,
		NodeContext: &context,
	}
}

func skippedMihomoProxyContext(item any, proxy map[string]any) domain.WarningNodeContext {
	if proxy == nil {
		return domain.WarningNodeContext{
			Format: "mihomo",
			Raw:    map[string]any{"value": item},
		}
	}
	port, _ := shared.Uint16Value(proxy["port"])
	return domain.WarningNodeContext{
		Format: "mihomo",
		Name:   shared.StringValue(proxy["name"]),
		Type:   mihomoNodeType(strings.ToLower(shared.StringValue(proxy["type"]))),
		Server: shared.StringValue(proxy["server"]),
		Port:   port,
		Raw:    proxy,
	}
}

func mihomoAliasConflictWarning(node domain.NodeIR, proxy map[string]any, nodeIndex int, alias, canonical string) domain.Warning {
	index := nodeIndex
	context := mihomoWarningNodeContext(node, proxy)
	return domain.Warning{
		Code:        "parse_alias_conflict",
		Message:     "legacy alias conflicts with " + canonical + "; canonical field takes precedence",
		Node:        node.Name,
		Source:      "mihomo",
		Field:       "mihomo." + alias,
		NodeIndex:   &index,
		NodeContext: &context,
	}
}
