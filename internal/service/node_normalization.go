package service

import (
	"crypto/sha1" //nolint:gosec // UUIDv5 is defined in terms of SHA-1; this is not used for security.
	"fmt"
	"strings"
	"uuid"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const defaultRealityClientFingerprint = "chrome"

func normalizeNodes(nodes []domain.NodeIR) []domain.NodeIR {
	out := append([]domain.NodeIR{}, nodes...)
	for index := range out {
		normalizeNodeUUID(&out[index])
		normalizeNodeVLESSFlow(&out[index])
		normalizeNodeRealityClientFingerprints(&out[index])
	}
	return out
}

func normalizeNodeVLESSFlow(node *domain.NodeIR) {
	if node == nil || node.Type != domain.NodeTypeVLESS {
		return
	}
	if strings.EqualFold(strings.TrimSpace(node.Flow), domain.VLESSFlowVision) {
		node.Flow = domain.VLESSFlowVision
	}
}

func normalizeNodeUUID(node *domain.NodeIR) {
	if node == nil || strings.TrimSpace(node.UUID) == "" {
		return
	}
	parsed, err := uuid.Parse(node.UUID)
	switch node.Type {
	case domain.NodeTypeVMess, domain.NodeTypeVLESS:
		if err != nil {
			parsed = uuidV5NilNamespace(node.UUID)
		}
		node.UUID = parsed.String()
	case domain.NodeTypeTUIC:
		if err == nil {
			node.UUID = parsed.String()
		}
	}
}

func uuidV5NilNamespace(name string) uuid.UUID {
	digest := sha1.Sum(append(make([]byte, len(uuid.UUID{})), name...))
	var id uuid.UUID
	copy(id[:], digest[:len(id)])
	id[6] = id[6]&0x0f | 0x50
	id[8] = id[8]&0x3f | 0x80
	return id
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
		normalizeNodeVLESSFlow(node)
		if node.Type != domain.NodeTypeVLESS ||
			!strings.EqualFold(strings.TrimSpace(node.Flow), domain.VLESSFlowVision) ||
			vlessVisionTransportCompatible(node.Transport) {
			continue
		}
		transportType := strings.TrimSpace(node.Transport.Type)
		node.Flow = ""
		nodeIndex := index
		node.Warnings = append(node.Warnings, domain.Warning{
			Code:      "node_normalized_incompatible_flow",
			Message:   fmt.Sprintf("removed VLESS Vision flow incompatible with %s transport", transportType),
			Node:      node.Name,
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
