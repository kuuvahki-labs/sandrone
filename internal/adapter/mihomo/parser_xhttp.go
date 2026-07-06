package mihomo

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func mihomoKnownXHTTPOptions() map[string]bool {
	return shared.KnownFields("mode", "reuse-settings", "download-settings")
}

func parseMihomoXHTTPReuseSettings(values map[string]any) *domain.XHTTPReuseSettings {
	if values == nil {
		return nil
	}
	return &domain.XHTTPReuseSettings{
		MaxConcurrency:   shared.StringValue(values["max-concurrency"]),
		MaxConnections:   shared.StringValue(values["max-connections"]),
		CMaxReuseTimes:   shared.StringValue(values["c-max-reuse-times"]),
		HMaxRequestTimes: shared.StringValue(values["h-max-request-times"]),
		HMaxReusableSecs: shared.StringValue(values["h-max-reusable-secs"]),
		HKeepAlivePeriod: intValueZero(values["h-keep-alive-period"]),
	}
}

func parseMihomoXHTTPDownloadSettings(values map[string]any) *domain.XHTTPDownloadSettings {
	if values == nil {
		return nil
	}
	out := &domain.XHTTPDownloadSettings{ReuseSettings: parseMihomoXHTTPReuseSettings(shared.AnyMapValue(values["reuse-settings"]))}
	if value, ok := values["server"]; ok {
		text := shared.StringValue(value)
		out.Server = &text
	}
	if value, ok := values["port"]; ok {
		port, _ := shared.Uint16Value(value)
		out.Port = &port
	}
	if value, ok := values["path"]; ok {
		text := shared.StringValue(value)
		out.Path = &text
	}
	if value, ok := values["host"]; ok {
		text := shared.StringValue(value)
		out.Host = &text
	}
	if value, ok := values["headers"]; ok {
		headers := shared.StringMapValue(value)
		out.Headers = &headers
	}
	if mihomoDownloadHasTLS(values) {
		out.TLS = parseMihomoDownloadTLS(values)
	}
	return out
}

func mihomoDownloadHasTLS(values map[string]any) bool {
	for _, key := range []string{"tls", "servername", "skip-cert-verify", "alpn", "client-fingerprint", "fingerprint", "certificate", "private-key", "ech-opts", "reality-opts"} {
		if _, ok := values[key]; ok {
			return true
		}
	}
	return false
}

func parseMihomoDownloadTLS(values map[string]any) *domain.TLSOptions {
	tls := &domain.TLSOptions{
		Enabled: shared.BoolValue(values["tls"]), ServerName: shared.StringValue(values["servername"]),
		InsecureSkipVerify: shared.BoolValue(values["skip-cert-verify"]), ALPN: shared.StringSliceValue(values["alpn"]),
		ClientFingerprint: shared.StringValue(values["client-fingerprint"]), Fingerprint: shared.StringValue(values["fingerprint"]),
		Certificate: shared.StringValue(values["certificate"]), PrivateKey: shared.StringValue(values["private-key"]),
	}
	if ech := shared.AnyMapValue(values["ech-opts"]); ech != nil {
		tls.ECH = &domain.ECHOptions{Enabled: shared.BoolValue(ech["enable"]), QueryServerName: shared.StringValue(ech["query-server-name"]), DNS: shared.StringValue(ech["_dns"]), ForceQuery: shared.StringValue(ech["_force-query"])}
		if config := shared.StringValue(ech["config"]); config != "" {
			tls.ECH.Config = []string{config}
		}
	}
	if reality := shared.AnyMapValue(values["reality-opts"]); reality != nil {
		tls.Enabled = true
		tls.Reality = &domain.RealityOptions{Enabled: true, PublicKey: shared.StringValue(reality["public-key"]), ShortID: shared.StringValue(reality["short-id"])}
	}
	return tls
}
