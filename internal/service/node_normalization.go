package service

import (
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const vlessVisionFlow = "xtls-rprx-vision"

func normalizeParsedNodes(parsed *parseInputResult) {
	if parsed == nil {
		return
	}
	for index := range parsed.Nodes {
		node := &parsed.Nodes[index]
		if node.Type != domain.NodeTypeVLESS ||
			!strings.EqualFold(strings.TrimSpace(node.Flow), vlessVisionFlow) ||
			vlessVisionTransportCompatible(node.Transport) {
			continue
		}
		transportType := strings.TrimSpace(node.Transport.Type)
		node.Flow = ""
		nodeIndex := index
		nodeID := node.ID
		if nodeID == "" {
			nodeID = node.Name
		}
		node.Warnings = append(node.Warnings, domain.Warning{
			Code:      "node_normalized_incompatible_flow",
			Message:   fmt.Sprintf("removed VLESS Vision flow incompatible with %s transport", transportType),
			Node:      nodeID,
			NodeIndex: &nodeIndex,
			Field:     "flow",
			Source:    "normalized",
		})
	}
}

func vlessVisionTransportCompatible(transport *domain.TransportOptions) bool {
	if transport == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(transport.Type)) {
	case "", "tcp", "raw":
		return true
	default:
		return false
	}
}
