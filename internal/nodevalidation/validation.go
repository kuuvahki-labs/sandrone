// Package nodevalidation validates normalized NodeIR semantics at service boundaries.
package nodevalidation

import (
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofrs/uuid/v5"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Stage string

const (
	StageNormalized Stage = "normalized"
	StageProcessed  Stage = "processed"
	StageRender     Stage = "render"
	StageProbe      Stage = "probe"
)

type Result struct {
	Nodes  []domain.NodeIR
	Issues []domain.ValidationIssue
	Counts domain.ValidationCounts
}

func Validate(nodes []domain.NodeIR, stage Stage, target string) Result {
	result := Result{
		Nodes:  make([]domain.NodeIR, 0, len(nodes)),
		Counts: domain.ValidationCounts{Input: len(nodes)},
	}
	for index, node := range nodes {
		issues := validateNode(node, index, stage, target)
		if len(issues) == 0 {
			result.Nodes = append(result.Nodes, node)
			result.Counts.Valid++
			continue
		}
		result.Counts.Invalid++
		for _, issue := range issues {
			if issue.Severity == "warning" {
				result.Counts.Warnings++
			} else {
				result.Counts.Errors++
			}
		}
		result.Issues = append(result.Issues, issues...)
	}
	return result
}

func validateNode(node domain.NodeIR, index int, stage Stage, target string) []domain.ValidationIssue {
	issues := make([]domain.ValidationIssue, 0, 4)
	add := func(code, field, message string) {
		nodeIndex := index
		issues = append(issues, domain.ValidationIssue{
			Severity:  "error",
			Stage:     string(stage),
			Code:      code,
			Message:   message,
			NodeIndex: &nodeIndex,
			RuntimeID: domain.NodeRuntimeID(node),
			NodeName:  node.Name,
			NodeType:  node.Type,
			Field:     field,
			Target:    target,
		})
	}
	required := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			add("node_validation_required", field, field+" is required")
		}
	}

	required("name", node.Name)
	endpointOnlyProbe := stage == StageProbe && node.Type == ""
	if !endpointOnlyProbe && !knownNodeType(node.Type) {
		add("node_validation_invalid", "type", "node type is not supported")
	}
	outerEndpointOptional := node.Type == domain.NodeTypeWireGuard && node.WireGuard != nil && len(node.WireGuard.Peers) > 0
	if !outerEndpointOptional && !validServer(node.Server) {
		add("node_validation_invalid", "server", "server must be a host or IP address without a scheme or path")
	}
	portRangeConfigured := node.Type == domain.NodeTypeMieru && node.Mieru != nil && strings.TrimSpace(node.Mieru.PortRange) != ""
	if !outerEndpointOptional && !portRangeConfigured && node.Port == 0 {
		add("node_validation_invalid", "port", "port must be between 1 and 65535")
	}
	if node.Network != "" && node.Network != "tcp" && node.Network != "udp" {
		add("node_validation_invalid", "network", "network must be tcp or udp")
	}

	switch node.Type {
	case domain.NodeTypeShadowsocks:
		required("cipher", node.Cipher)
		required("password", node.Password)
	case domain.NodeTypeShadowsocksR:
		required("cipher", node.Cipher)
		required("password", node.Password)
		if node.ShadowsocksR == nil {
			add("node_validation_required", "shadowsocksr", "shadowsocksr options are required")
		}
	case domain.NodeTypeVMess, domain.NodeTypeVLESS:
		if !validUUID(node.UUID) {
			add("node_validation_invalid", "uuid", "uuid must be a valid UUID")
		}
		if node.Type == domain.NodeTypeVLESS {
			validateVLESS(node, add)
		}
	case domain.NodeTypeTrojan:
		required("password", node.Password)
	case domain.NodeTypeHysteria, domain.NodeTypeHysteria2:
		if node.TLS == nil || !node.TLS.Enabled {
			add("node_validation_required", "tls", "TLS must be enabled")
		}
		if node.Type == domain.NodeTypeHysteria2 && node.Hysteria != nil && node.Hysteria.Obfs != "" && strings.TrimSpace(node.Hysteria.ObfsPassword) == "" {
			add("node_validation_required", "hysteria.obfs_password", "obfuscation password is required when obfuscation is enabled")
		}
	case domain.NodeTypeTUIC:
		validateTUIC(node, add)
		if node.TLS == nil || !node.TLS.Enabled {
			add("node_validation_required", "tls", "TLS must be enabled")
		}
	case domain.NodeTypeMieru:
		required("username", node.Username)
		required("password", node.Password)
		if node.Mieru == nil {
			add("node_validation_required", "mieru", "mieru options are required")
		}
	case domain.NodeTypeSOCKS, domain.NodeTypeHTTP:
		if node.Password != "" && node.Username == "" {
			add("node_validation_conflict", "username", "username is required when password is set")
		}
	case domain.NodeTypeWireGuard:
		if node.WireGuard == nil {
			add("node_validation_required", "wireguard", "wireguard options are required")
		} else {
			validateWireGuard(node.WireGuard, add)
		}
	case domain.NodeTypeSnell:
		required("password", node.Password)
		switch {
		case node.Snell == nil:
			add("node_validation_required", "snell", "snell options are required")
		case node.Snell.Version < 1 || node.Snell.Version > 5:
			add("node_validation_invalid", "snell.version", "snell version must be between 1 and 5")
		default:
			validateSnell(node.Snell, add)
		}
	case domain.NodeTypeAnyTLS:
		required("password", node.Password)
		if node.TLS == nil || !node.TLS.Enabled {
			add("node_validation_required", "tls", "TLS must be enabled")
		}
		if node.AnyTLS == nil {
			add("node_validation_required", "anytls", "anytls options are required")
		} else {
			validateAnyTLSDuration(node.AnyTLS.IdleSessionCheckInterval, "anytls.idle_session_check_interval", add)
			validateAnyTLSDuration(node.AnyTLS.IdleSessionTimeout, "anytls.idle_session_timeout", add)
			if node.AnyTLS.MinIdleSession < 0 {
				add("node_validation_invalid", "anytls.min_idle_session", "minimum idle sessions must not be negative")
			}
		}
	}
	validateTLS(node.TLS, "tls", add)
	validateTransport(node.Transport, add)
	return issues
}

func validateVLESS(node domain.NodeIR, add func(string, string, string)) {
	switch node.Flow {
	case "", domain.VLESSFlowVision:
	default:
		add("node_validation_invalid", "flow", "VLESS flow is not supported")
	}
}

func validateTLS(options *domain.TLSOptions, prefix string, add func(string, string, string)) {
	if options == nil {
		return
	}
	if !validTLSClientFingerprint(options.ClientFingerprint) {
		add("node_validation_invalid", prefix+".client_fingerprint", "TLS client fingerprint is not supported")
	}
	if (options.Certificate == "") != (options.PrivateKey == "") {
		add("node_validation_conflict", prefix+".certificate", "TLS certificate and private key must be configured together")
	}
	if options.ECH != nil {
		switch options.ECH.ForceQuery {
		case "", "none", "half", "full":
		default:
			add("node_validation_invalid", prefix+".ech.force_query", "ECH force query must be none, half, or full")
		}
	}
}

func validTLSClientFingerprint(value string) bool {
	switch value {
	case "",
		"chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized",
		"chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk":
		return true
	default:
		return false
	}
}

func validateTransport(transport *domain.TransportOptions, add func(string, string, string)) {
	if transport == nil || transport.Type != "xhttp" || transport.XHTTP == nil {
		return
	}
	validateXHTTPReuse(transport.XHTTP.ReuseSettings, "transport.xhttp.reuse_settings", add)
	download := transport.XHTTP.DownloadSettings
	if download == nil {
		return
	}
	if download.Server != nil && !validServer(*download.Server) {
		add("node_validation_invalid", "transport.xhttp.download_settings.server", "download server must be a host or IP address")
	}
	if download.Port != nil && *download.Port == 0 {
		add("node_validation_invalid", "transport.xhttp.download_settings.port", "download port must be between 1 and 65535")
	}
	validateTLS(download.TLS, "transport.xhttp.download_settings.tls", add)
	validateXHTTPReuse(download.ReuseSettings, "transport.xhttp.download_settings.reuse_settings", add)
}

func validateXHTTPReuse(options *domain.XHTTPReuseSettings, prefix string, add func(string, string, string)) {
	if options == nil {
		return
	}
	values := []struct {
		field string
		value string
	}{
		{"max_concurrency", options.MaxConcurrency},
		{"max_connections", options.MaxConnections},
		{"c_max_reuse_times", options.CMaxReuseTimes},
		{"h_max_request_times", options.HMaxRequestTimes},
		{"h_max_reusable_secs", options.HMaxReusableSecs},
	}
	for _, item := range values {
		if item.value != "" && !validNonNegativeRange(item.value) {
			add("node_validation_invalid", prefix+"."+item.field, "xHTTP reuse value must be a non-negative integer or ascending range")
		}
	}
	if options.HKeepAlivePeriod < 0 {
		add("node_validation_invalid", prefix+".h_keep_alive_period", "xHTTP keep-alive period must not be negative")
	}
}

func validNonNegativeRange(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	numbers := make([]int64, len(parts))
	for index, part := range parts {
		number, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || number < 0 {
			return false
		}
		numbers[index] = number
	}
	return len(numbers) == 1 || numbers[0] <= numbers[1]
}

func validateSnell(options *domain.SnellOptions, add func(string, string, string)) {
	if options.Reuse != nil {
		switch options.Version {
		case 1, 3:
			add("node_validation_invalid", "snell.reuse", "this Snell version does not support reuse")
		case 2:
			if !*options.Reuse {
				add("node_validation_invalid", "snell.reuse", "Snell v2 always enables reuse")
			}
		}
	}
	if options.ShadowTLS == nil {
		return
	}
	if options.Obfs != "" || options.ObfsHost != "" {
		add("node_validation_conflict", "snell.obfs", "ordinary obfs and ShadowTLS cannot be configured together")
	}
	shadow := options.ShadowTLS
	if strings.TrimSpace(shadow.Password) == "" {
		add("node_validation_required", "snell.shadow_tls.password", "ShadowTLS password is required")
	}
	if strings.TrimSpace(shadow.Host) == "" {
		add("node_validation_required", "snell.shadow_tls.host", "ShadowTLS host is required")
	}
	if shadow.Version < 0 || shadow.Version > 3 {
		add("node_validation_invalid", "snell.shadow_tls.version", "ShadowTLS version must be between 1 and 3, or 0 for the default")
	}
	if (shadow.Certificate == "") != (shadow.PrivateKey == "") {
		add("node_validation_conflict", "snell.shadow_tls.certificate", "ShadowTLS certificate and private key must be configured together")
	}
}

func knownNodeType(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeShadowsocks, domain.NodeTypeShadowsocksR, domain.NodeTypeVMess,
		domain.NodeTypeVLESS, domain.NodeTypeTrojan, domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2, domain.NodeTypeTUIC, domain.NodeTypeMieru,
		domain.NodeTypeSOCKS, domain.NodeTypeHTTP, domain.NodeTypeWireGuard,
		domain.NodeTypeSnell, domain.NodeTypeAnyTLS:
		return true
	default:
		return false
	}
}

func validateAnyTLSDuration(value string, field string, add func(string, string, string)) {
	if strings.TrimSpace(value) == "" {
		return
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 || duration%time.Second != 0 {
		add("node_validation_invalid", field, "duration must be a positive whole number of seconds")
	}
}

func validServer(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "://") || strings.ContainsAny(value, "/?#") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")) != nil
	}
	if strings.Contains(value, ":") {
		return net.ParseIP(value) != nil
	}
	return true
}

func validateTUIC(node domain.NodeIR, add func(string, string, string)) {
	hasToken := strings.TrimSpace(node.Token) != ""
	hasUUID := strings.TrimSpace(node.UUID) != ""
	hasPassword := strings.TrimSpace(node.Password) != ""
	if hasToken == (hasUUID || hasPassword) {
		add("node_validation_conflict", "tuic.credentials", "TUIC requires exactly one credential mode")
	}
	if hasUUID != hasPassword {
		add("node_validation_conflict", "tuic.credentials", "TUIC UUID and password must be provided together")
	}
	if hasUUID && !validUUID(node.UUID) {
		add("node_validation_invalid", "uuid", "uuid must be a valid UUID")
	}
}

func validUUID(value string) bool {
	_, err := uuid.FromString(value)
	return err == nil
}

func validateWireGuard(options *domain.WireGuardOptions, add func(string, string, string)) {
	if strings.TrimSpace(options.PrivateKey) == "" {
		add("node_validation_required", "wireguard.private_key", "wireguard private key is required")
	}
	if len(options.Address) == 0 && strings.TrimSpace(options.IP) == "" && strings.TrimSpace(options.IPv6) == "" {
		add("node_validation_required", "wireguard.address", "at least one wireguard address is required")
	}
	for _, address := range options.Address {
		if _, _, err := net.ParseCIDR(address); err != nil {
			add("node_validation_invalid", "wireguard.address", "wireguard address must use CIDR notation")
			break
		}
	}
	for _, peer := range options.Peers {
		if strings.TrimSpace(peer.PublicKey) == "" {
			add("node_validation_required", "wireguard.peers.public_key", "wireguard peer public key is required")
			break
		}
	}
}
