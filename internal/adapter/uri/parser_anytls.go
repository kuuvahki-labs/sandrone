package uri

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseAnyTLS(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeAnyTLS, SourceFormat: "uri"}
	source := shared.SourceInfo("anytls", shared.SourceRefs("anytls"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse anytls URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "443")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse anytls server", err)
	}
	password := u.User.Username()
	if password == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing anytls password")
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.Password = password
	values := u.Query()
	minIdle, _ := strconv.Atoi(values.Get("min-idle-session"))
	node.AnyTLS = &domain.AnyTLSOptions{
		IdleSessionCheckInterval: values.Get("idle-session-check-interval"),
		IdleSessionTimeout:       values.Get("idle-session-timeout"),
		MinIdleSession:           minIdle,
	}
	applyTLSQuery(&node, values)
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	node.Raw = map[string]json.RawMessage{}
	known := map[string]bool{
		"idle-session-check-interval": true, "idle-session-timeout": true, "min-idle-session": true,
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true, "alpn": true,
		"allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"pbk": true, "public-key": true, "sid": true, "short-id": true,
	}
	if queryValuesEqualFold(values, "type", "tcp") {
		known["type"] = true
	}
	if queryValuesAreNoopHeaderType(values) {
		known["headerType"] = true
	}
	preserveURIQuery(&node, values, known)
	return node, source, nil
}
