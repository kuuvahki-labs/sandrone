package uri

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type xhttpExtraWire struct {
	XMux             *xhttpReuseWire    `json:"xmux,omitempty"`
	DownloadSettings *xhttpDownloadWire `json:"downloadSettings,omitempty"`
}

type xhttpReuseWire struct {
	MaxConcurrency   string `json:"maxConcurrency,omitempty"`
	MaxConnections   string `json:"maxConnections,omitempty"`
	CMaxReuseTimes   string `json:"cMaxReuseTimes,omitempty"`
	HMaxRequestTimes string `json:"hMaxRequestTimes,omitempty"`
	HMaxReusableSecs string `json:"hMaxReusableSecs,omitempty"`
	HKeepAlivePeriod int    `json:"hKeepAlivePeriod,omitempty"`
}

type xhttpDownloadWire struct {
	Address         *string            `json:"address,omitempty"`
	Port            *uint16            `json:"port,omitempty"`
	Network         string             `json:"network,omitempty"`
	Security        string             `json:"security,omitempty"`
	XHTTPSettings   *xhttpSettingsWire `json:"xhttpSettings,omitempty"`
	TLSSettings     *xhttpTLSWire      `json:"tlsSettings,omitempty"`
	RealitySettings *xhttpRealityWire  `json:"realitySettings,omitempty"`
}

type xhttpSettingsWire struct {
	Path  *string         `json:"path,omitempty"`
	Host  *string         `json:"host,omitempty"`
	Extra *xhttpExtraWire `json:"extra,omitempty"`
}

type xhttpTLSWire struct {
	ServerName    string   `json:"serverName,omitempty"`
	AllowInsecure bool     `json:"allowInsecure,omitempty"`
	ALPN          []string `json:"alpn,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	ECHConfigList []string `json:"echConfigList,omitempty"`
	ECHQuery      string   `json:"echQuery,omitempty"`
	ECHDNS        string   `json:"echDNS,omitempty"`
	ECHForceQuery string   `json:"echForceQuery,omitempty"`
}

type xhttpRealityWire struct {
	PublicKey string `json:"publicKey,omitempty"`
	ShortID   string `json:"shortId,omitempty"`
}

func applyVLESSXHTTPExtra(transport *domain.TransportOptions, values url.Values) {
	if transport == nil || transport.Type != "xhttp" {
		return
	}
	transport.XHTTP = &domain.XHTTPTransportOptions{Mode: values.Get("mode")}
	raw := values.Get("extra")
	if raw == "" {
		return
	}
	var extra xhttpExtraWire
	if json.Unmarshal([]byte(raw), &extra) != nil {
		return
	}
	transport.XHTTP.ReuseSettings = reuseFromWire(extra.XMux)
	if extra.DownloadSettings != nil {
		transport.XHTTP.DownloadSettings = downloadFromWire(extra.DownloadSettings)
	}
}

func reuseFromWire(wire *xhttpReuseWire) *domain.XHTTPReuseSettings {
	if wire == nil {
		return nil
	}
	return &domain.XHTTPReuseSettings{
		MaxConcurrency: wire.MaxConcurrency, MaxConnections: wire.MaxConnections,
		CMaxReuseTimes: wire.CMaxReuseTimes, HMaxRequestTimes: wire.HMaxRequestTimes,
		HMaxReusableSecs: wire.HMaxReusableSecs, HKeepAlivePeriod: wire.HKeepAlivePeriod,
	}
}

func downloadFromWire(wire *xhttpDownloadWire) *domain.XHTTPDownloadSettings {
	download := &domain.XHTTPDownloadSettings{Server: wire.Address, Port: wire.Port}
	if wire.XHTTPSettings != nil {
		download.Path = wire.XHTTPSettings.Path
		download.Host = wire.XHTTPSettings.Host
		if wire.XHTTPSettings.Extra != nil {
			download.ReuseSettings = reuseFromWire(wire.XHTTPSettings.Extra.XMux)
		}
	}
	if wire.Security == "tls" || wire.TLSSettings != nil {
		tls := &domain.TLSOptions{Enabled: true}
		if settings := wire.TLSSettings; settings != nil {
			tls.ServerName = settings.ServerName
			tls.InsecureSkipVerify = settings.AllowInsecure
			tls.ALPN = settings.ALPN
			tls.ClientFingerprint = settings.Fingerprint
			if len(settings.ECHConfigList) > 0 || settings.ECHQuery != "" || settings.ECHDNS != "" || settings.ECHForceQuery != "" {
				tls.ECH = &domain.ECHOptions{Enabled: true, Config: settings.ECHConfigList, QueryServerName: settings.ECHQuery, DNS: settings.ECHDNS, ForceQuery: settings.ECHForceQuery}
			}
		}
		download.TLS = tls
	}
	if wire.Security == "reality" || wire.RealitySettings != nil {
		if download.TLS == nil {
			download.TLS = &domain.TLSOptions{Enabled: true}
		}
		download.TLS.Reality = &domain.RealityOptions{Enabled: true}
		if reality := wire.RealitySettings; reality != nil {
			download.TLS.Reality.PublicKey = reality.PublicKey
			download.TLS.Reality.ShortID = reality.ShortID
		}
	}
	return download
}

func renderVLESSXHTTPExtra(transport *domain.TransportOptions) string {
	if transport == nil || transport.XHTTP == nil {
		return ""
	}
	extra := xhttpExtraWire{XMux: reuseToWire(transport.XHTTP.ReuseSettings)}
	if download := transport.XHTTP.DownloadSettings; download != nil {
		extra.DownloadSettings = downloadToWire(download)
	}
	body, err := json.Marshal(extra)
	if err != nil || string(body) == "{}" {
		return ""
	}
	return string(body)
}

func reuseToWire(options *domain.XHTTPReuseSettings) *xhttpReuseWire {
	if options == nil {
		return nil
	}
	return &xhttpReuseWire{
		MaxConcurrency: options.MaxConcurrency, MaxConnections: options.MaxConnections,
		CMaxReuseTimes: options.CMaxReuseTimes, HMaxRequestTimes: options.HMaxRequestTimes,
		HMaxReusableSecs: options.HMaxReusableSecs, HKeepAlivePeriod: options.HKeepAlivePeriod,
	}
}

func downloadToWire(download *domain.XHTTPDownloadSettings) *xhttpDownloadWire {
	wire := &xhttpDownloadWire{
		Address: download.Server, Port: download.Port, Network: "xhttp",
		XHTTPSettings: &xhttpSettingsWire{Path: download.Path, Host: download.Host, Extra: &xhttpExtraWire{XMux: reuseToWire(download.ReuseSettings)}},
	}
	if wire.XHTTPSettings.Extra.XMux == nil {
		wire.XHTTPSettings.Extra = nil
	}
	if tls := download.TLS; tls != nil {
		wire.Security = "tls"
		wire.TLSSettings = &xhttpTLSWire{
			ServerName: tls.ServerName, AllowInsecure: tls.InsecureSkipVerify,
			ALPN: tls.ALPN, Fingerprint: tls.ClientFingerprint,
		}
		if tls.ECH != nil {
			wire.TLSSettings.ECHConfigList = tls.ECH.Config
			wire.TLSSettings.ECHQuery = tls.ECH.QueryServerName
			wire.TLSSettings.ECHDNS = tls.ECH.DNS
			wire.TLSSettings.ECHForceQuery = tls.ECH.ForceQuery
		}
		if tls.Reality != nil {
			wire.Security = "reality"
			wire.RealitySettings = &xhttpRealityWire{PublicKey: tls.Reality.PublicKey, ShortID: tls.Reality.ShortID}
		}
	}
	return wire
}

func parseECHQuery(value, forceQuery string) *domain.ECHOptions {
	if value == "" && forceQuery == "" {
		return nil
	}
	options := &domain.ECHOptions{Enabled: true, ForceQuery: forceQuery}
	for _, marker := range []string{"+https://", "+tls://"} {
		if queryServerName, dnsSuffix, ok := strings.Cut(value, marker); ok {
			options.QueryServerName = queryServerName
			options.DNS = strings.TrimPrefix(marker, "+") + dnsSuffix
			return options
		}
	}
	if strings.Contains(value, "://") {
		options.DNS = value
	} else if value != "" {
		options.Config = splitList(value)
	}
	return options
}

func renderECHQuery(options *domain.ECHOptions) string {
	if options == nil {
		return ""
	}
	if options.DNS != "" {
		if options.QueryServerName != "" {
			return options.QueryServerName + "+" + options.DNS
		}
		return options.DNS
	}
	return strings.Join(options.Config, ",")
}
