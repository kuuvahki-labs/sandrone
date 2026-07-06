package shadowrocket

import (
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type tlsFields struct {
	enabled    bool
	serverName bool
	insecure   bool
	alpn       bool
}

func validateTLS(tls *domain.TLSOptions, supported tlsFields) error {
	if tls == nil {
		return nil
	}
	if tls.Enabled && !supported.enabled {
		return domain.NewError(domain.CodeRenderFailed, "TLS is not documented for this Shadowrocket local proxy protocol")
	}
	if tls.ServerName != "" && !supported.serverName {
		return domain.NewError(domain.CodeRenderFailed, "TLS server name is not documented for this Shadowrocket local proxy protocol")
	}
	if tls.InsecureSkipVerify && !supported.insecure {
		return domain.NewError(domain.CodeRenderFailed, "TLS verification override is not documented for this Shadowrocket local proxy protocol")
	}
	if len(tls.ALPN) > 0 {
		if !supported.alpn || len(tls.ALPN) != 1 {
			return domain.NewError(domain.CodeRenderFailed, "TLS ALPN cannot be represented by the documented Shadowrocket local proxy syntax")
		}
		if err := safeScalar("tls.alpn", tls.ALPN[0]); err != nil {
			return err
		}
	}
	if tls.Certificate != "" || tls.PrivateKey != "" {
		return domain.NewError(domain.CodeRenderFailed, "TLS client identity is not documented for Shadowrocket local proxy output")
	}
	if tls.DisableSNI {
		return domain.NewError(domain.CodeRenderFailed, "disabling SNI is not documented for Shadowrocket local proxy output")
	}
	if tls.ECH != nil {
		return domain.NewError(domain.CodeRenderFailed, "ECH is not documented for Shadowrocket local proxy output")
	}
	if tls.Reality != nil {
		return domain.NewError(domain.CodeRenderFailed, "Reality is not documented for Shadowrocket local proxy output")
	}
	return nil
}

func appendOptionalTLS(parts []string, tls *domain.TLSOptions, includeTLS, includeInsecure bool) ([]string, error) {
	if tls == nil {
		return parts, nil
	}
	if includeTLS && tls.Enabled {
		parts = append(parts, "tls=true")
	}
	var err error
	parts, err = appendOptionalField(parts, "peer", "tls.server_name", tls.ServerName)
	if err != nil {
		return nil, err
	}
	if includeInsecure && tls.InsecureSkipVerify {
		parts = append(parts, "allowInsecure=1")
	}
	return parts, nil
}

func appendTLSIdentity(parts []string, tls *domain.TLSOptions, includeALPN bool) ([]string, error) {
	if tls == nil {
		return parts, nil
	}
	var err error
	parts, err = appendOptionalField(parts, "peer", "tls.server_name", tls.ServerName)
	if err != nil {
		return nil, err
	}
	if includeALPN && len(tls.ALPN) == 1 {
		parts, err = appendOptionalField(parts, "alpn", "tls.alpn", tls.ALPN[0])
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func appendCredentials(parts []string, username, password string) ([]string, error) {
	if username == "" && password == "" {
		return parts, nil
	}
	if username == "" {
		return nil, domain.NewError(domain.CodeRenderFailed, "username is required when password is set")
	}
	if err := safeScalar("username", username); err != nil {
		return nil, err
	}
	if err := safeScalar("password", password); err != nil {
		return nil, err
	}
	return append(parts, username, password), nil
}

func appendUDP(parts []string, node domain.NodeIR) []string {
	if node.Dialer != nil && node.Dialer.UDPRelay != nil && *node.Dialer.UDPRelay {
		return append(parts, "udp=1")
	}
	return parts
}

func documentedTransport(transport *domain.TransportOptions, websocketAllowed bool) (bool, error) {
	if transport == nil {
		return false, nil
	}
	typeName := strings.ToLower(strings.TrimSpace(transport.Type))
	hasDetails := transport.Method != "" || transport.Path != "" || transport.Host != "" ||
		len(transport.Hosts) > 0 || len(transport.Headers) > 0 || transport.ServiceName != "" ||
		transport.MaxEarlyData != 0 || transport.EarlyDataHeaderName != "" ||
		transport.V2RayHTTPUpgrade || transport.V2RayHTTPUpgradeFastOpen || transport.XHTTP != nil
	if typeName == "" || typeName == "tcp" {
		if hasDetails {
			return false, domain.NewError(domain.CodeRenderFailed, "transport details cannot be represented by the documented Shadowrocket local proxy syntax")
		}
		return false, nil
	}
	if websocketAllowed && (typeName == "websocket" || typeName == "ws") && !hasDetails {
		return true, nil
	}
	return false, domain.NewError(domain.CodeRenderFailed, "transport is not documented for this Shadowrocket local proxy protocol")
}

func isDefaultTCPTransport(transport *domain.TransportOptions) bool {
	if transport == nil {
		return false
	}
	typeName := strings.ToLower(strings.TrimSpace(transport.Type))
	return typeName == "" || typeName == "tcp"
}

func markTransportEmitted(emitted emittedFields, transport *domain.TransportOptions, websocket bool) {
	if transport != nil && (websocket || isDefaultTCPTransport(transport)) {
		emitted["transport.type"] = true
	}
}

func markTLSEmitted(emitted emittedFields, tls *domain.TLSOptions, insecure, serverName, alpn bool) {
	if tls == nil {
		return
	}
	emitted["tls.enabled"] = true
	if serverName && tls.ServerName != "" {
		emitted["tls.server_name"] = true
	}
	if insecure && tls.InsecureSkipVerify {
		emitted["tls.insecure_skip_verify"] = true
	}
	if alpn && len(tls.ALPN) > 0 {
		emitted["tls.alpn"] = true
	}
}

func markUDPEmitted(emitted emittedFields, node domain.NodeIR) {
	if node.Dialer != nil && node.Dialer.UDPRelay != nil {
		emitted["dialer.udp_relay"] = true
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func joinReserved(values []uint8) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(int(value)))
	}
	return strings.Join(parts, "/")
}

func protocolLossWarning(node domain.NodeIR, field string) domain.Warning {
	return shared.RenderLossyWarning(node, rendererName, field, "")
}
