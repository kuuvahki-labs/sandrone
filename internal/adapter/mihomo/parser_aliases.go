package mihomo

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseMihomoDialer(node *domain.NodeIR, proxy map[string]any, nodeIndex int) []domain.Warning {
	warnings := []domain.Warning{}
	if _, ok := proxy["udp"]; ok && mihomoSupportsUDPRelay(node.Type) {
		if node.Dialer == nil {
			node.Dialer = &domain.DialerOptions{}
		}
		udp := shared.BoolValue(proxy["udp"])
		node.Dialer.UDPRelay = &udp
	}
	fastOpen := node.Type == domain.NodeTypeHysteria || node.Type == domain.NodeTypeTUIC
	legacyVMessFastOpen := node.Type == domain.NodeTypeVMess
	_, hasTFO := proxy["tfo"]
	_, hasFastOpen := proxy["fast-open"]
	if legacyVMessFastOpen && hasTFO && hasFastOpen && shared.BoolValue(proxy["tfo"]) != shared.BoolValue(proxy["fast-open"]) {
		warnings = append(warnings, mihomoAliasConflictWarning(*node, proxy, nodeIndex, "fast-open", "tfo"))
	}
	useLegacyVMessFastOpen := legacyVMessFastOpen && !hasTFO && shared.BoolValue(proxy["fast-open"])
	if shared.BoolValue(proxy["tfo"]) || (fastOpen && shared.BoolValue(proxy["fast-open"])) || useLegacyVMessFastOpen {
		if node.Dialer == nil {
			node.Dialer = &domain.DialerOptions{}
		}
		node.Dialer.TFO = true
	}
	return warnings
}

func parseMihomoShadowsocksR(node *domain.NodeIR, proxy map[string]any, nodeIndex int) []domain.Warning {
	warnings := []domain.Warning{}
	protocolParam, hasProtocolParam := proxy["protocol-param"]
	legacyProtocolParam, hasLegacyProtocolParam := proxy["protocolparam"]
	if !hasProtocolParam {
		protocolParam = legacyProtocolParam
	} else if hasLegacyProtocolParam && shared.StringValue(protocolParam) != shared.StringValue(legacyProtocolParam) {
		warnings = append(warnings, mihomoAliasConflictWarning(*node, proxy, nodeIndex, "protocolparam", "protocol-param"))
	}
	obfsParam, hasObfsParam := proxy["obfs-param"]
	legacyObfsParam, hasLegacyObfsParam := proxy["obfsparam"]
	if !hasObfsParam {
		obfsParam = legacyObfsParam
	} else if hasLegacyObfsParam && shared.StringValue(obfsParam) != shared.StringValue(legacyObfsParam) {
		warnings = append(warnings, mihomoAliasConflictWarning(*node, proxy, nodeIndex, "obfsparam", "obfs-param"))
	}
	node.ShadowsocksR = &domain.ShadowsocksROptions{
		Protocol:      shared.StringValue(proxy["protocol"]),
		ProtocolParam: shared.StringValue(protocolParam),
		Obfs:          shared.StringValue(proxy["obfs"]),
		ObfsParam:     shared.StringValue(obfsParam),
	}
	return warnings
}
