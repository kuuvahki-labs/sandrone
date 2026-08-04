package nodevalidation_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

func TestValidateKeepsValidNodesAndReportsInvalidNodesWithoutSecrets(t *testing.T) {
	t.Parallel()

	result := nodevalidation.Validate([]domain.NodeIR{
		{
			ID:       "valid",
			Name:     "valid",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     443,
			Cipher:   "aes-128-gcm",
			Password: "valid-secret",
		},
		{
			ID:       "invalid",
			Name:     "invalid",
			Type:     domain.NodeTypeTrojan,
			Server:   "https://bad.example/path",
			Port:     0,
			Password: "must-not-leak",
		},
	}, nodevalidation.StageNormalized, "mihomo-proxies")

	require.Len(t, result.Nodes, 1)
	require.Equal(t, "valid", result.Nodes[0].ID)
	require.Equal(t, domain.ValidationCounts{
		Input:   2,
		Valid:   1,
		Invalid: 1,
		Errors:  2,
	}, result.Counts)
	require.Len(t, result.Issues, 2)
	require.Equal(t, "invalid", result.Issues[0].NodeID)
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
