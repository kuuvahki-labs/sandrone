package nodevalidation_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

const validRealityPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestValidateKeepsValidNodesAndReportsInvalidNodesWithoutSecrets(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{
			Name:     "valid",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     443,
			Cipher:   "aes-128-gcm",
			Password: "valid-secret",
		},
		{
			Name:     "invalid",
			Type:     domain.NodeTypeTrojan,
			Server:   "https://bad.example/path",
			Port:     0,
			Password: "must-not-leak",
		},
	}
	domain.AssignNodeRuntimeIDs(nodes)
	result := nodevalidation.Validate(nodes, nodevalidation.StageNormalized, "mihomo-proxies")

	require.Len(t, result.Nodes, 1)
	require.NotEmpty(t, domain.NodeRuntimeID(result.Nodes[0]))
	require.Equal(t, domain.ValidationCounts{
		Input:   2,
		Valid:   1,
		Invalid: 1,
		Errors:  2,
	}, result.Counts)
	require.Len(t, result.Issues, 2)
	require.NotEmpty(t, result.Issues[0].RuntimeID)
	require.Equal(t, domain.NodeTypeTrojan, result.Issues[0].NodeType)
	require.Equal(t, "mihomo-proxies", result.Issues[0].Target)

	encoded, err := json.Marshal(result.Issues)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "must-not-leak")
	require.NotContains(t, string(encoded), "https://bad.example/path")
}

func TestValidateRejectsProtocolSpecificRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		node  domain.NodeIR
		field string
	}{
		{
			name:  "shadowsocks cipher",
			node:  domain.NodeIR{Name: "ss", Type: domain.NodeTypeShadowsocks, Server: "example.com", Port: 443, Password: "secret"},
			field: "cipher",
		},
		{
			name:  "vmess uuid",
			node:  domain.NodeIR{Name: "vmess", Type: domain.NodeTypeVMess, Server: "example.com", Port: 443},
			field: "uuid",
		},
		{
			name:  "wireguard options",
			node:  domain.NodeIR{Name: "wg", Type: domain.NodeTypeWireGuard, Server: "example.com", Port: 51820},
			field: "wireguard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := nodevalidation.Validate([]domain.NodeIR{tt.node}, nodevalidation.StageNormalized, "")
			require.Empty(t, result.Nodes)
			require.Equal(t, 1, result.Counts.Invalid)
			require.Contains(t, issueFields(result.Issues), tt.field)
		})
	}
}

func TestValidateVLESSFlow(t *testing.T) {
	t.Parallel()
	base := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111",
	}
	tests := []struct {
		name  string
		node  domain.NodeIR
		valid bool
		field string
	}{
		{name: "vision", node: func() domain.NodeIR {
			node := base
			node.Flow = domain.VLESSFlowVision
			return node
		}(), valid: true},
		{name: "unknown flow", node: func() domain.NodeIR {
			node := base
			node.Flow = "future-flow"
			return node
		}(), field: "flow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := nodevalidation.Validate([]domain.NodeIR{tt.node}, nodevalidation.StageNormalized, "")
			if tt.valid {
				require.Equal(t, 1, result.Counts.Valid)
				return
			}
			require.Zero(t, result.Counts.Valid)
			require.Contains(t, issueFields(result.Issues), tt.field)
		})
	}
}

func TestValidateTLSClientFingerprint(t *testing.T) {
	t.Parallel()

	base := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
	}
	valid := []string{
		"", "chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized",
		"chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk",
	}
	for _, fingerprint := range valid {
		fingerprint := fingerprint
		t.Run("valid_"+fingerprint, func(t *testing.T) {
			t.Parallel()
			node := base
			node.TLS = &domain.TLSOptions{Enabled: true, ClientFingerprint: fingerprint}
			require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "").Counts.Valid)
		})
	}

	for _, fingerprint := range []string{"unsafe", "chrome120", "none", " Chrome"} {
		fingerprint := fingerprint
		t.Run("invalid_"+fingerprint, func(t *testing.T) {
			t.Parallel()
			node := base
			node.TLS = &domain.TLSOptions{Enabled: true, ClientFingerprint: fingerprint}
			result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "")
			require.Equal(t, 1, result.Counts.Invalid)
			require.Equal(t, []string{"tls.client_fingerprint"}, issueFields(result.Issues))
		})
	}
}

func TestValidateRealityPublicKey(t *testing.T) {
	t.Parallel()

	base := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
	}
	tests := []struct {
		name string
		key  string
		code string
	}{
		{name: "valid", key: validRealityPublicKey},
		{name: "missing", code: "node_validation_required"},
		{name: "invalid encoding", key: "not+a-reality-key", code: "node_validation_invalid"},
		{name: "wrong length", key: "c2hvcnQ", code: "node_validation_invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := base
			node.TLS = &domain.TLSOptions{
				Enabled: true,
				Reality: &domain.RealityOptions{Enabled: true, PublicKey: tt.key},
			}

			result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "")
			if tt.code == "" {
				require.Equal(t, 1, result.Counts.Valid)
				return
			}
			require.Equal(t, 1, result.Counts.Invalid)
			require.Len(t, result.Issues, 1)
			require.Equal(t, tt.code, result.Issues[0].Code)
			require.Equal(t, "tls.reality.public_key", result.Issues[0].Field)
		})
	}
}

func TestValidateXHTTPDownloadTLSClientFingerprint(t *testing.T) {
	t.Parallel()

	node := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			DownloadSettings: &domain.XHTTPDownloadSettings{
				TLS: &domain.TLSOptions{Enabled: true, ClientFingerprint: "unsafe"},
			},
		}},
	}

	result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "")

	require.Equal(t, 1, result.Counts.Invalid)
	require.Equal(t, []string{"transport.xhttp.download_settings.tls.client_fingerprint"}, issueFields(result.Issues))
}

func TestValidateTUICRequiresEnabledTLS(t *testing.T) {
	t.Parallel()

	validCredentials := domain.NodeIR{
		Name: "tuic", Type: domain.NodeTypeTUIC, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Password: "secret",
	}
	tests := []struct {
		name string
		tls  *domain.TLSOptions
	}{
		{name: "missing"},
		{name: "disabled", tls: &domain.TLSOptions{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := validCredentials
			node.TLS = tt.tls

			result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "")

			require.Empty(t, result.Nodes)
			require.Equal(t, 1, result.Counts.Invalid)
			require.Len(t, result.Issues, 1)
			require.Equal(t, "node_validation_required", result.Issues[0].Code)
			require.Equal(t, "tls", result.Issues[0].Field)
		})
	}
}

func TestValidateUUIDsWithStandardParserAndKeepsTUICStrict(t *testing.T) {
	t.Parallel()

	compactVMess := domain.NodeIR{
		Name: "vmess", Type: domain.NodeTypeVMess, Server: "vmess.example", Port: 443,
		UUID: "11111111111111111111111111111111", Cipher: "auto",
	}
	require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{compactVMess}, nodevalidation.StageNormalized, "").Counts.Valid)

	invalidVLESS := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "vless.example", Port: 443,
		UUID: "not-normalized", Encryption: "none",
	}
	require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{invalidVLESS}, nodevalidation.StageNormalized, "").Counts.Invalid)

	invalidTUIC := domain.NodeIR{
		Name: "tuic", Type: domain.NodeTypeTUIC, Server: "tuic.example", Port: 443,
		UUID: "not-a-uuid", Password: "secret", TLS: &domain.TLSOptions{Enabled: true},
	}
	result := nodevalidation.Validate([]domain.NodeIR{invalidTUIC}, nodevalidation.StageNormalized, "")
	require.Equal(t, 1, result.Counts.Invalid)
	require.Contains(t, issueFields(result.Issues), "uuid")
}

func TestValidateRejectsNonCanonicalNetwork(t *testing.T) {
	t.Parallel()

	node := domain.NodeIR{
		Name:     "trojan-ws",
		Type:     domain.NodeTypeTrojan,
		Server:   "example.com",
		Port:     443,
		Password: "secret",
		Network:  "ws",
		TLS:      &domain.TLSOptions{Enabled: true},
		Transport: &domain.TransportOptions{
			Type: "websocket",
		},
	}

	result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageProbe, "sing-box")

	require.Empty(t, result.Nodes)
	require.Equal(t, 1, result.Counts.Invalid)
	require.Len(t, result.Issues, 1)
	require.Equal(t, "network", result.Issues[0].Field)
	require.Equal(t, "node_validation_invalid", result.Issues[0].Code)
}

func TestValidateAnyTLSRequiresTLSAndWholeSecondDurations(t *testing.T) {
	t.Parallel()

	valid := domain.NodeIR{
		Name: "anytls", Type: domain.NodeTypeAnyTLS, Server: "example.com", Port: 443,
		Password: "secret", TLS: &domain.TLSOptions{Enabled: true},
		AnyTLS: &domain.AnyTLSOptions{IdleSessionCheckInterval: "30s", IdleSessionTimeout: "5m", MinIdleSession: 1},
	}
	require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{valid}, nodevalidation.StageNormalized, "").Counts.Valid)

	invalid := valid
	invalid.TLS = nil
	invalid.AnyTLS = &domain.AnyTLSOptions{IdleSessionCheckInterval: "1500ms", IdleSessionTimeout: "bad", MinIdleSession: -1}
	result := nodevalidation.Validate([]domain.NodeIR{invalid}, nodevalidation.StageNormalized, "")
	require.Equal(t, 1, result.Counts.Invalid)
	require.ElementsMatch(t, []string{"tls", "anytls.idle_session_check_interval", "anytls.idle_session_timeout", "anytls.min_idle_session"}, issueFields(result.Issues))
}

func TestValidateSnellReuseAndShadowTLSVersions(t *testing.T) {
	t.Parallel()

	validReuse := false
	valid := domain.NodeIR{
		Name: "snell", Type: domain.NodeTypeSnell, Server: "example.com", Port: 44046, Password: "secret",
		Snell: &domain.SnellOptions{Version: 5, Reuse: &validReuse, ShadowTLS: &domain.ShadowTLSOptions{
			Password: "shadow", Host: "cdn.example.com", Version: 3,
		}},
	}
	require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{valid}, nodevalidation.StageNormalized, "").Counts.Valid)

	invalidReuse := false
	invalid := valid
	invalid.Snell = &domain.SnellOptions{
		Version: 2, Reuse: &invalidReuse, Obfs: "tls", ObfsHost: "cdn.example.com",
		ShadowTLS: &domain.ShadowTLSOptions{Version: 4},
	}
	result := nodevalidation.Validate([]domain.NodeIR{invalid}, nodevalidation.StageNormalized, "")
	require.Equal(t, 1, result.Counts.Invalid)
	require.ElementsMatch(t, []string{
		"snell.reuse", "snell.obfs", "snell.shadow_tls.password", "snell.shadow_tls.host", "snell.shadow_tls.version",
	}, issueFields(result.Issues))
}

func TestValidateXHTTPReuseRangesAndECHForceQuery(t *testing.T) {
	t.Parallel()

	server := "download.example.com"
	port := uint16(8443)
	valid := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111",
		TLS:  &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, DNS: "https://1.1.1.1/dns-query", ForceQuery: "full"}},
		Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			ReuseSettings: &domain.XHTTPReuseSettings{MaxConcurrency: "8-16", HKeepAlivePeriod: 15},
			DownloadSettings: &domain.XHTTPDownloadSettings{Server: &server, Port: &port,
				ReuseSettings: &domain.XHTTPReuseSettings{MaxConnections: "2"}},
		}},
	}
	require.Equal(t, 1, nodevalidation.Validate([]domain.NodeIR{valid}, nodevalidation.StageNormalized, "").Counts.Valid)

	invalid := valid
	badServer := "https://bad.example/path"
	zeroPort := uint16(0)
	invalid.TLS = &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, ForceQuery: "always"}}
	invalid.Transport = &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
		ReuseSettings: &domain.XHTTPReuseSettings{MaxConcurrency: "16-8", HKeepAlivePeriod: -1},
		DownloadSettings: &domain.XHTTPDownloadSettings{Server: &badServer, Port: &zeroPort,
			ReuseSettings: &domain.XHTTPReuseSettings{MaxConnections: "many"}},
	}}
	result := nodevalidation.Validate([]domain.NodeIR{invalid}, nodevalidation.StageNormalized, "")
	require.Equal(t, 1, result.Counts.Invalid)
	require.ElementsMatch(t, []string{
		"tls.ech.force_query", "transport.xhttp.reuse_settings.max_concurrency",
		"transport.xhttp.reuse_settings.h_keep_alive_period", "transport.xhttp.download_settings.server",
		"transport.xhttp.download_settings.port", "transport.xhttp.download_settings.reuse_settings.max_connections",
	}, issueFields(result.Issues))
}

func issueFields(issues []domain.ValidationIssue) []string {
	fields := make([]string, 0, len(issues))
	for _, issue := range issues {
		fields = append(fields, issue.Field)
	}
	return fields
}
