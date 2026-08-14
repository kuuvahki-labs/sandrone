package service

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid/v5"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const vlessVisionFlow = "xtls-rprx-vision"
const defaultRealityClientFingerprint = "chrome"

func normalizeNodes(nodes []domain.NodeIR) []domain.NodeIR {
	out := append([]domain.NodeIR{}, nodes...)
	for index := range out {
		normalizeNodeUUID(&out[index])
		normalizeNodeRealityClientFingerprints(&out[index])
	}
	return out
}

func normalizeNodeUUID(node *domain.NodeIR) {
	if node == nil || strings.TrimSpace(node.UUID) == "" {
		return
	}
	parsed, err := uuid.FromString(node.UUID)
	switch node.Type {
	case domain.NodeTypeVMess, domain.NodeTypeVLESS:
		if err != nil {
			parsed = uuid.NewV5(uuid.Nil, node.UUID)
		}
		node.UUID = parsed.String()
	case domain.NodeTypeTUIC:
		if err == nil {
			node.UUID = parsed.String()
		}
	}
}

func normalizeNodeRealityClientFingerprints(node *domain.NodeIR) {
	if node == nil {
		return
	}
	node.TLS = normalizeRealityClientFingerprint(node.TLS)
	if node.Transport == nil || node.Transport.XHTTP == nil || node.Transport.XHTTP.DownloadSettings == nil {
		return
	}
	downloadTLS := node.Transport.XHTTP.DownloadSettings.TLS
	normalizedDownloadTLS := normalizeRealityClientFingerprint(downloadTLS)
	if normalizedDownloadTLS == downloadTLS {
		return
	}
	transport := *node.Transport
	xhttp := *transport.XHTTP
	download := *xhttp.DownloadSettings
	download.TLS = normalizedDownloadTLS
	xhttp.DownloadSettings = &download
	transport.XHTTP = &xhttp
	node.Transport = &transport
}

func normalizeRealityClientFingerprint(options *domain.TLSOptions) *domain.TLSOptions {
	if options == nil || options.Reality == nil || strings.TrimSpace(options.ClientFingerprint) != "" {
		return options
	}
	normalized := *options
	normalized.ClientFingerprint = defaultRealityClientFingerprint
	return &normalized
}

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
