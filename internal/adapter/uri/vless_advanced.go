package uri

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type xhttpExtraWire struct {
	XMux             *xhttpReuseWire    `json:"xmux,omitzero"`
	DownloadSettings *xhttpDownloadWire `json:"downloadSettings,omitzero"`
}

type xhttpReuseWire struct {
	MaxConcurrency   string `json:"maxConcurrency,omitempty"`
	MaxConnections   string `json:"maxConnections,omitempty"`
	CMaxReuseTimes   string `json:"cMaxReuseTimes,omitempty"`
	HMaxRequestTimes string `json:"hMaxRequestTimes,omitempty"`
	HMaxReusableSecs string `json:"hMaxReusableSecs,omitempty"`
	HKeepAlivePeriod int    `json:"hKeepAlivePeriod,omitzero"`
}

type xhttpDownloadWire struct {
	Address         *string            `json:"address,omitzero"`
	Port            *uint16            `json:"port,omitzero"`
	Network         string             `json:"network,omitempty"`
	Security        string             `json:"security,omitempty"`
	XHTTPSettings   *xhttpSettingsWire `json:"xhttpSettings,omitzero"`
	TLSSettings     *xhttpTLSWire      `json:"tlsSettings,omitzero"`
	RealitySettings *xhttpRealityWire  `json:"realitySettings,omitzero"`
}

type xhttpSettingsWire struct {
	Path  *string         `json:"path,omitzero"`
	Host  *string         `json:"host,omitzero"`
	Extra *xhttpExtraWire `json:"extra,omitzero"`
}

type xhttpTLSWire struct {
	ServerName    string   `json:"serverName,omitempty"`
	AllowInsecure bool     `json:"allowInsecure,omitzero"`
	ALPN          []string `json:"alpn,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	ECHConfigList []string `json:"echConfigList,omitempty"`
	ECHQuery      string   `json:"echQuery,omitempty"`
	ECHDNS        string   `json:"echDNS,omitempty"`
	ECHForceQuery string   `json:"echForceQuery,omitempty"`
}

type xhttpRealityWire struct {
	PublicKey     string `json:"publicKey,omitempty"`
	ShortID       string `json:"shortId,omitempty"`
	MLDSA65Verify string `json:"mldsa65Verify,omitempty"`
	SpiderX       string `json:"spiderX,omitempty"`
}

func applyXHTTPExtra(transport *domain.TransportOptions, values url.Values) bool {
	raw, ok := prepareXHTTPExtra(transport, values)
	if !ok {
		return false
	}
	var extra xhttpExtraWire
	if json.Unmarshal([]byte(raw), &extra) != nil {
		return false
	}
	promoteXHTTPExtraFromWire(transport.XHTTP, &extra)
	return xhttpExtraComplete(raw)
}

func applyVMessXHTTPExtra(transport *domain.TransportOptions, values url.Values) bool {
	raw, ok := prepareXHTTPExtra(transport, values)
	if !ok {
		return false
	}
	var extra xhttpExtraWire
	if json.Unmarshal([]byte(raw), &extra) == nil && !jsonContainsNull(raw) {
		promoteXHTTPExtraFromWire(transport.XHTTP, &extra)
		return xhttpExtraComplete(raw)
	}
	if fields, ok := jsonObjectFields([]byte(raw)); ok {
		if xmux, exists := fields["xmux"]; exists {
			transport.XHTTP.ReuseSettings = reuseFromJSON(xmux)
		}
		if download, exists := fields["downloadSettings"]; exists {
			transport.XHTTP.DownloadSettings = downloadFromJSON(download)
		}
	}
	return xhttpExtraComplete(raw)
}

func promoteXHTTPExtraFromWire(options *domain.XHTTPTransportOptions, extra *xhttpExtraWire) {
	options.ReuseSettings = reuseFromWire(extra.XMux)
	if extra.DownloadSettings != nil {
		options.DownloadSettings = downloadFromWire(extra.DownloadSettings)
	}
}

func prepareXHTTPExtra(transport *domain.TransportOptions, values url.Values) (string, bool) {
	if transport == nil || transport.Type != "xhttp" {
		return "", false
	}
	transport.XHTTP = &domain.XHTTPTransportOptions{Mode: values.Get("mode")}
	raw := values.Get("extra")
	return raw, raw != ""
}

func xhttpExtraComplete(raw string) bool {
	var complete *xhttpExtraWire
	if err := json.Unmarshal([]byte(raw), &complete, json.RejectUnknownMembers(true)); err != nil {
		return false
	}
	if complete == nil {
		return false
	}
	if jsonContainsNull(raw) {
		return false
	}
	return xhttpExtraValuesComplete(complete)
}

func xhttpExtraValuesComplete(extra *xhttpExtraWire) bool {
	if extra.DownloadSettings == nil {
		return true
	}
	download := extra.DownloadSettings
	if download.Network != "xhttp" {
		return false
	}
	if download.XHTTPSettings != nil &&
		download.XHTTPSettings.Extra != nil &&
		download.XHTTPSettings.Extra.DownloadSettings != nil {
		return false
	}
	switch download.Security {
	case "", "none":
		return download.TLSSettings == nil && download.RealitySettings == nil
	case "tls":
		return download.TLSSettings != nil && download.RealitySettings == nil
	case "reality":
		return download.RealitySettings != nil && download.TLSSettings == nil
	default:
		return false
	}
}

func jsonContainsNull(raw string) bool {
	decoder := jsontext.NewDecoder(strings.NewReader(raw))
	for {
		token, err := decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return false
		}
		if token.Kind() == 'n' {
			return true
		}
	}
}

func jsonObjectFields(raw []byte) (map[string]jsontext.Value, bool) {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false
	}
	return fields, true
}

func decodeJSONField[T any](fields map[string]jsontext.Value, name string) (T, bool) {
	var value T
	raw, ok := fields[name]
	if !ok || jsonContainsNull(string(raw)) {
		return value, false
	}
	if json.Unmarshal(raw, &value) != nil {
		return value, false
	}
	return value, true
}

func reuseFromJSON(raw jsontext.Value) *domain.XHTTPReuseSettings {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return nil
	}
	reuse := &domain.XHTTPReuseSettings{}
	promoted := len(fields) == 0
	if value, ok := decodeJSONField[string](fields, "maxConcurrency"); ok {
		reuse.MaxConcurrency = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "maxConnections"); ok {
		reuse.MaxConnections = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "cMaxReuseTimes"); ok {
		reuse.CMaxReuseTimes = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "hMaxRequestTimes"); ok {
		reuse.HMaxRequestTimes = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "hMaxReusableSecs"); ok {
		reuse.HMaxReusableSecs = value
		promoted = true
	}
	if value, ok := decodeJSONField[int](fields, "hKeepAlivePeriod"); ok {
		reuse.HKeepAlivePeriod = value
		promoted = true
	}
	if !promoted {
		return nil
	}
	return reuse
}

func downloadFromJSON(raw jsontext.Value) *domain.XHTTPDownloadSettings {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return nil
	}
	download := &domain.XHTTPDownloadSettings{}
	promoted := len(fields) == 0
	if value, ok := decodeJSONField[string](fields, "address"); ok {
		download.Server = &value
		promoted = true
	}
	if value, ok := decodeJSONField[uint16](fields, "port"); ok {
		download.Port = &value
		promoted = true
	}
	if _, ok := decodeJSONField[string](fields, "network"); ok {
		promoted = true
	}
	security, hasSecurity := decodeJSONField[string](fields, "security")
	if hasSecurity {
		promoted = true
	}
	if settingsRaw, exists := fields["xhttpSettings"]; exists {
		if applyDownloadXHTTPSettingsFromJSON(download, settingsRaw) {
			promoted = true
		}
	}

	var tls *domain.TLSOptions
	if settingsRaw, exists := fields["tlsSettings"]; exists {
		if settings := tlsFromJSON(settingsRaw); settings != nil {
			tls = settings
			promoted = true
		}
	}
	if hasSecurity && security == "tls" {
		if tls == nil {
			tls = &domain.TLSOptions{Enabled: true}
		}
		promoted = true
	}

	var reality *domain.RealityOptions
	if settingsRaw, exists := fields["realitySettings"]; exists {
		if settings := realityFromJSON(settingsRaw); settings != nil {
			reality = settings
			promoted = true
		}
	}
	if hasSecurity && security == "reality" {
		if reality == nil {
			reality = &domain.RealityOptions{Enabled: true}
		}
		promoted = true
	}
	if reality != nil {
		if tls == nil {
			tls = &domain.TLSOptions{Enabled: true}
		}
		tls.Reality = reality
	}
	download.TLS = tls

	if !promoted {
		return nil
	}
	return download
}

func applyDownloadXHTTPSettingsFromJSON(download *domain.XHTTPDownloadSettings, raw jsontext.Value) bool {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return false
	}
	promoted := len(fields) == 0
	if value, ok := decodeJSONField[string](fields, "path"); ok {
		download.Path = &value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "host"); ok {
		download.Host = &value
		promoted = true
	}
	if extraRaw, exists := fields["extra"]; exists {
		if extraFields, ok := jsonObjectFields(extraRaw); ok {
			if xmuxRaw, exists := extraFields["xmux"]; exists {
				if reuse := reuseFromJSON(xmuxRaw); reuse != nil {
					download.ReuseSettings = reuse
					promoted = true
				}
			}
		}
	}
	return promoted
}

func tlsFromJSON(raw jsontext.Value) *domain.TLSOptions {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return nil
	}
	tls := &domain.TLSOptions{Enabled: true}
	promoted := len(fields) == 0
	if value, ok := decodeJSONField[string](fields, "serverName"); ok {
		tls.ServerName = value
		promoted = true
	}
	if value, ok := decodeJSONField[bool](fields, "allowInsecure"); ok {
		tls.InsecureSkipVerify = value
		promoted = true
	}
	if value, ok := decodeJSONField[[]string](fields, "alpn"); ok {
		tls.ALPN = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "fingerprint"); ok {
		tls.ClientFingerprint = value
		promoted = true
	}

	ech := &domain.ECHOptions{Enabled: true}
	if value, ok := decodeJSONField[[]string](fields, "echConfigList"); ok {
		ech.Config = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "echQuery"); ok {
		ech.QueryServerName = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "echDNS"); ok {
		ech.DNS = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "echForceQuery"); ok {
		ech.ForceQuery = value
		promoted = true
	}
	if len(ech.Config) > 0 || ech.QueryServerName != "" || ech.DNS != "" || ech.ForceQuery != "" {
		tls.ECH = ech
	}
	if !promoted {
		return nil
	}
	return tls
}

func realityFromJSON(raw jsontext.Value) *domain.RealityOptions {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return nil
	}
	reality := &domain.RealityOptions{Enabled: true}
	promoted := len(fields) == 0
	if value, ok := decodeJSONField[string](fields, "publicKey"); ok {
		reality.PublicKey = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "shortId"); ok {
		reality.ShortID = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "mldsa65Verify"); ok {
		reality.MLDSA65Verify = value
		promoted = true
	}
	if value, ok := decodeJSONField[string](fields, "spiderX"); ok {
		reality.SpiderX = value
		promoted = true
	}
	if !promoted {
		return nil
	}
	return reality
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
			download.TLS.Reality.MLDSA65Verify = reality.MLDSA65Verify
			download.TLS.Reality.SpiderX = reality.SpiderX
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
	body, err := json.Marshal(extra, json.Deterministic(true))
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
			wire.RealitySettings = &xhttpRealityWire{
				PublicKey: tls.Reality.PublicKey, ShortID: tls.Reality.ShortID,
				MLDSA65Verify: tls.Reality.MLDSA65Verify, SpiderX: tls.Reality.SpiderX,
			}
		}
	}
	return wire
}

func parseECHQuery(value, forceQuery string) *domain.ECHOptions {
	if value == "" && forceQuery == "" {
		return nil
	}
	options := &domain.ECHOptions{Enabled: true, ForceQuery: forceQuery}
	for _, marker := range []string{"+https://", "+tls://", "+udp://"} {
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
