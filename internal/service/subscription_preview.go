package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	subscriptionPreviewStatusAdded     = "added"
	subscriptionPreviewStatusModified  = "modified"
	subscriptionPreviewStatusRemoved   = "removed"
	subscriptionPreviewStatusUnchanged = "unchanged"
)

func (s *Service) PreviewSubscription(ctx context.Context, name string, args ...map[string]string) (*domain.SubscriptionPreviewResult, error) {
	return s.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{
		Name:    name,
		Request: domain.RequestInfo{Args: optionalRequestArgs(args...)},
	})
}

func (s *Service) PreviewSubscriptionRequest(ctx context.Context, req domain.SubscriptionPreviewRequest) (*domain.SubscriptionPreviewResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}
	sub, err := s.metaStore.GetSubscription(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	before, after, err := s.subscriptionPreviewNodes(ctx, sub, domain.FileRequest{
		Request: domain.RequestInfo{Args: req.Request.Args, Meta: sub.Meta},
		Meta:    sub.Meta,
	}, newSubscriptionResolveState())
	if err != nil {
		return nil, err
	}
	report := domain.Report{
		Dependencies: append([]domain.ResourceRef{}, after.Dependencies...),
		Warnings:     append([]domain.Warning{}, after.Warnings...),
	}
	for _, source := range after.Sources {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
	}
	report = s.prepareReport("subscription_preview", report)

	nodes, counts := diffPreviewNodes(before.Nodes, after.Nodes)
	attachPreviewTargetNames(nodes, "shadowrocket", shadowrocket.PreviewNodeNames(after.Nodes))
	return &domain.SubscriptionPreviewResult{
		SubscriptionName: sub.Name,
		Type:             sub.Type,
		Format:           sub.Format,
		BeforeCount:      len(before.Nodes),
		AfterCount:       len(after.Nodes),
		StatusCounts:     counts,
		Nodes:            nodes,
		Report:           report,
	}, nil
}

func attachPreviewTargetNames(diffs []domain.SubscriptionPreviewNodeDiff, target string, names []string) {
	afterIndex := 0
	for index := range diffs {
		if diffs[index].After == nil {
			continue
		}
		if afterIndex >= len(names) {
			return
		}
		diffs[index].TargetNames = map[string]string{target: names[afterIndex]}
		afterIndex++
	}
}

func diffPreviewNodes(before, after []domain.NodeIR) ([]domain.SubscriptionPreviewNodeDiff, map[string]int) {
	counts := map[string]int{
		subscriptionPreviewStatusAdded:     0,
		subscriptionPreviewStatusModified:  0,
		subscriptionPreviewStatusRemoved:   0,
		subscriptionPreviewStatusUnchanged: 0,
	}
	beforeIdentities := make([]string, len(before))
	for index, node := range before {
		beforeIdentities[index] = previewNodeIdentity(node)
	}
	afterIdentities := make([]string, len(after))
	for index, node := range after {
		afterIdentities[index] = previewNodeIdentity(node)
	}

	ops := previewDiffOps(before, after, beforeIdentities, afterIdentities)
	diffs := make([]domain.SubscriptionPreviewNodeDiff, 0, len(ops))
	for _, op := range ops {
		switch op.Kind {
		case previewDiffOpMatch:
			beforeNode := before[op.BeforeIndex]
			afterNode := after[op.AfterIndex]
			status := subscriptionPreviewStatusUnchanged
			if !previewNodeNativeEqual(beforeNode, afterNode) {
				status = subscriptionPreviewStatusModified
			}
			counts[status]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				Identity: beforeIdentities[op.BeforeIndex],
				Status:   status,
				Before:   clonePreviewNode(beforeNode),
				After:    clonePreviewNode(afterNode),
			})
		case previewDiffOpRemove:
			counts[subscriptionPreviewStatusRemoved]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				Identity: beforeIdentities[op.BeforeIndex],
				Status:   subscriptionPreviewStatusRemoved,
				Before:   clonePreviewNode(before[op.BeforeIndex]),
			})
		case previewDiffOpAdd:
			counts[subscriptionPreviewStatusAdded]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				Identity: afterIdentities[op.AfterIndex],
				Status:   subscriptionPreviewStatusAdded,
				After:    clonePreviewNode(after[op.AfterIndex]),
			})
		}
	}
	return diffs, counts
}

func previewDiffOps(before, after []domain.NodeIR, beforeIdentities, afterIdentities []string) []previewDiffStep {
	beforeToAfter, afterToBefore := matchPreviewNodes(before, after, beforeIdentities, afterIdentities)
	steps := make([]previewDiffStep, 0, max(len(before), len(after)))
	removedEmitted := make([]bool, len(before))
	emitRemovedBefore := func(limit int) {
		for beforeIndex := 0; beforeIndex < limit; beforeIndex++ {
			if beforeToAfter[beforeIndex] != -1 || removedEmitted[beforeIndex] {
				continue
			}
			steps = append(steps, previewDiffStep{Kind: previewDiffOpRemove, BeforeIndex: beforeIndex})
			removedEmitted[beforeIndex] = true
		}
	}
	for afterIndex := range after {
		beforeIndex := afterToBefore[afterIndex]
		if beforeIndex == -1 {
			steps = append(steps, previewDiffStep{Kind: previewDiffOpAdd, AfterIndex: afterIndex})
			continue
		}
		emitRemovedBefore(beforeIndex)
		steps = append(steps, previewDiffStep{Kind: previewDiffOpMatch, BeforeIndex: beforeIndex, AfterIndex: afterIndex})
	}
	emitRemovedBefore(len(before))
	return steps
}

func matchPreviewNodes(before, after []domain.NodeIR, beforeIdentities, afterIdentities []string) ([]int, []int) {
	beforeToAfter := make([]int, len(before))
	for index := range beforeToAfter {
		beforeToAfter[index] = -1
	}
	afterToBefore := make([]int, len(after))
	for index := range afterToBefore {
		afterToBefore[index] = -1
	}
	for beforeIndex, beforeNode := range before {
		for afterIndex, afterNode := range after {
			if afterToBefore[afterIndex] != -1 || beforeIdentities[beforeIndex] != afterIdentities[afterIndex] {
				continue
			}
			if previewNodeNativeEqual(beforeNode, afterNode) {
				beforeToAfter[beforeIndex] = afterIndex
				afterToBefore[afterIndex] = beforeIndex
				break
			}
		}
	}
	for beforeIndex := range before {
		if beforeToAfter[beforeIndex] != -1 {
			continue
		}
		for afterIndex := range after {
			if afterToBefore[afterIndex] != -1 || beforeIdentities[beforeIndex] != afterIdentities[afterIndex] {
				continue
			}
			beforeToAfter[beforeIndex] = afterIndex
			afterToBefore[afterIndex] = beforeIndex
			break
		}
	}
	return beforeToAfter, afterToBefore
}

func previewNodeNativeEqual(left, right domain.NodeIR) bool {
	return reflect.DeepEqual(previewNodeNativeFields(left), previewNodeNativeFields(right))
}

func previewNodeNativeFields(node domain.NodeIR) domain.NodeIR {
	node.ID = ""
	node.Tags = nil
	node.Meta = nil
	node.Lossy = false
	node.Warnings = nil
	node.SourceFormat = ""
	node.Hysteria = previewNativeHysteria(node.Hysteria)
	return node
}

type previewDiffOp byte

const (
	previewDiffOpRemove previewDiffOp = iota + 1
	previewDiffOpAdd
	previewDiffOpMatch
)

type previewDiffStep struct {
	Kind        previewDiffOp
	BeforeIndex int
	AfterIndex  int
}

func clonePreviewNode(node domain.NodeIR) *domain.NodeIR {
	cloned := node
	return &cloned
}

func previewNodeIdentity(node domain.NodeIR) string {
	body, err := json.Marshal(previewNodeConnectionIdentity{ //nolint:gosec // Preview identity intentionally includes credential fields so credential changes affect node diff identity; the payload is hashed immediately and not exposed.
		Type:                 string(node.Type),
		Server:               strings.ToLower(node.Server),
		Port:                 node.Port,
		Network:              node.Network,
		Username:             node.Username,
		Password:             node.Password,
		UUID:                 node.UUID,
		Cipher:               node.Cipher,
		Flow:                 node.Flow,
		Encryption:           node.Encryption,
		Token:                node.Token,
		PacketEncoding:       node.PacketEncoding,
		Plugin:               node.Plugin,
		Path:                 node.Path,
		ShadowsocksR:         node.ShadowsocksR,
		Snell:                node.Snell,
		AnyTLS:               node.AnyTLS,
		Transport:            previewTransportIdentity(node.Transport),
		Hysteria:             previewHysteriaIdentity(node.Hysteria),
		TUIC:                 node.TUIC,
		Mieru:                node.Mieru,
		WireGuard:            previewWireGuardIdentity(node.WireGuard),
		TLSServerName:        previewTLSServerName(node.TLS),
		RealityPublicKey:     previewRealityPublicKey(node.TLS),
		RealityShortID:       previewRealityShortID(node.TLS),
		UDPOverTCP:           node.UDPOverTCP,
		PluginOptionsDigest:  stableAnyDigest(node.PluginOptions),
		HeadersDigest:        stableStringMapDigest(node.Headers),
		TransportHeaderHash:  stableStringMapDigest(transportHeaders(node.Transport)),
		WireGuardPeerKeyHash: stableWireGuardPeerDigest(node.WireGuard),
	})
	if err != nil {
		body = []byte(string(node.Type) + "|" + node.Server + "|" + strconv.Itoa(int(node.Port)))
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type previewNodeConnectionIdentity struct {
	Type                 string                      `json:"type,omitempty"`
	Server               string                      `json:"server,omitempty"`
	Port                 uint16                      `json:"port,omitempty"`
	Network              string                      `json:"network,omitempty"`
	Username             string                      `json:"username,omitempty"`
	Password             string                      `json:"password,omitempty"`
	UUID                 string                      `json:"uuid,omitempty"`
	Cipher               string                      `json:"cipher,omitempty"`
	Flow                 string                      `json:"flow,omitempty"`
	Encryption           string                      `json:"encryption,omitempty"`
	Token                string                      `json:"token,omitempty"`
	PacketEncoding       string                      `json:"packet_encoding,omitempty"`
	Plugin               string                      `json:"plugin,omitempty"`
	Path                 string                      `json:"path,omitempty"`
	ShadowsocksR         *domain.ShadowsocksROptions `json:"shadowsocksr,omitempty"`
	Snell                *domain.SnellOptions        `json:"snell,omitempty"`
	AnyTLS               *domain.AnyTLSOptions       `json:"anytls,omitempty"`
	Transport            *previewTransport           `json:"transport,omitempty"`
	Hysteria             *previewHysteria            `json:"hysteria,omitempty"`
	TUIC                 *domain.TUICOptions         `json:"tuic,omitempty"`
	Mieru                *domain.MieruOptions        `json:"mieru,omitempty"`
	WireGuard            *previewWireGuard           `json:"wireguard,omitempty"`
	TLSServerName        string                      `json:"tls_server_name,omitempty"`
	RealityPublicKey     string                      `json:"reality_public_key,omitempty"`
	RealityShortID       string                      `json:"reality_short_id,omitempty"`
	UDPOverTCP           *domain.UDPOverTCPOptions   `json:"udp_over_tcp,omitempty"`
	PluginOptionsDigest  string                      `json:"plugin_options_digest,omitempty"`
	HeadersDigest        string                      `json:"headers_digest,omitempty"`
	TransportHeaderHash  string                      `json:"transport_header_hash,omitempty"`
	WireGuardPeerKeyHash string                      `json:"wireguard_peer_key_hash,omitempty"`
}

type previewTransport struct {
	Type        string                        `json:"type,omitempty"`
	Method      string                        `json:"method,omitempty"`
	Path        string                        `json:"path,omitempty"`
	Host        string                        `json:"host,omitempty"`
	Hosts       []string                      `json:"hosts,omitempty"`
	ServiceName string                        `json:"service_name,omitempty"`
	XHTTP       *domain.XHTTPTransportOptions `json:"xhttp,omitempty"`
}

type previewHysteria struct {
	Protocol     string   `json:"protocol,omitempty"`
	ServerPorts  []string `json:"server_ports,omitempty"`
	Auth         string   `json:"auth,omitempty"`
	AuthString   string   `json:"auth_str,omitempty"`
	Obfs         string   `json:"obfs,omitempty"`
	ObfsPassword string   `json:"obfs_password,omitempty"`
}

type previewWireGuard struct {
	PrivateKey string   `json:"private_key,omitempty"`
	Address    []string `json:"address,omitempty"`
	IP         string   `json:"ip,omitempty"`
	IPv6       string   `json:"ipv6,omitempty"`
}

func previewTransportIdentity(transport *domain.TransportOptions) *previewTransport {
	if transport == nil {
		return nil
	}
	return &previewTransport{
		Type:        transport.Type,
		Method:      transport.Method,
		Path:        transport.Path,
		Host:        transport.Host,
		Hosts:       append([]string{}, transport.Hosts...),
		ServiceName: transport.ServiceName,
		XHTTP:       transport.XHTTP,
	}
}

func previewHysteriaIdentity(hysteria *domain.HysteriaOptions) *previewHysteria {
	if hysteria == nil {
		return nil
	}
	out := &previewHysteria{
		Protocol:     hysteria.Protocol,
		ServerPorts:  append([]string{}, hysteria.ServerPorts...),
		Auth:         hysteria.Auth,
		AuthString:   hysteria.AuthString,
		Obfs:         hysteria.Obfs,
		ObfsPassword: hysteria.ObfsPassword,
	}
	if previewHysteriaEmpty(out) {
		return nil
	}
	return out
}

func previewHysteriaEmpty(hysteria *previewHysteria) bool {
	return hysteria == nil ||
		hysteria.Protocol == "" &&
			len(hysteria.ServerPorts) == 0 &&
			hysteria.Auth == "" &&
			hysteria.AuthString == "" &&
			hysteria.Obfs == "" &&
			hysteria.ObfsPassword == ""
}

func previewNativeHysteria(hysteria *domain.HysteriaOptions) *domain.HysteriaOptions {
	if hysteria == nil || reflect.DeepEqual(*hysteria, domain.HysteriaOptions{}) {
		return nil
	}
	return hysteria
}

func previewWireGuardIdentity(wireguard *domain.WireGuardOptions) *previewWireGuard {
	if wireguard == nil {
		return nil
	}
	return &previewWireGuard{
		PrivateKey: wireguard.PrivateKey,
		Address:    append([]string{}, wireguard.Address...),
		IP:         wireguard.IP,
		IPv6:       wireguard.IPv6,
	}
}

func previewTLSServerName(tls *domain.TLSOptions) string {
	if tls == nil {
		return ""
	}
	return tls.ServerName
}

func previewRealityPublicKey(tls *domain.TLSOptions) string {
	if tls == nil || tls.Reality == nil {
		return ""
	}
	return tls.Reality.PublicKey
}

func previewRealityShortID(tls *domain.TLSOptions) string {
	if tls == nil || tls.Reality == nil {
		return ""
	}
	return tls.Reality.ShortID
}

func transportHeaders(transport *domain.TransportOptions) map[string]string {
	if transport == nil {
		return nil
	}
	return transport.Headers
}

func stableStringMapDigest(value map[string]string) string {
	if len(value) == 0 {
		return ""
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+value[key])
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func stableAnyDigest(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func stableWireGuardPeerDigest(wireguard *domain.WireGuardOptions) string {
	if wireguard == nil || len(wireguard.Peers) == 0 {
		return ""
	}
	type peerIdentity struct {
		Server       string `json:"server,omitempty"`
		Port         uint16 `json:"port,omitempty"`
		PublicKey    string `json:"public_key,omitempty"`
		PreSharedKey string `json:"pre_shared_key,omitempty"`
	}
	peers := make([]peerIdentity, 0, len(wireguard.Peers))
	for _, peer := range wireguard.Peers {
		peers = append(peers, peerIdentity{
			Server:       strings.ToLower(peer.Server),
			Port:         peer.Port,
			PublicKey:    peer.PublicKey,
			PreSharedKey: peer.PreSharedKey,
		})
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Server != peers[j].Server {
			return peers[i].Server < peers[j].Server
		}
		if peers[i].Port != peers[j].Port {
			return peers[i].Port < peers[j].Port
		}
		return peers[i].PublicKey < peers[j].PublicKey
	})
	body, err := json.Marshal(peers)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
