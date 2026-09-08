package domain_test

import (
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeRuntimeIDIsUniqueAndRuntimeOnly(t *testing.T) {
	nodes := []domain.NodeIR{{Name: "one"}, {Name: "two"}}
	domain.AssignNodeRuntimeIDs(nodes)
	require.NotEmpty(t, domain.NodeRuntimeID(nodes[0]))
	require.NotEqual(t, domain.NodeRuntimeID(nodes[0]), domain.NodeRuntimeID(nodes[1]))

	jsonBody, err := json.Marshal(nodes[0])
	require.NoError(t, err)
	require.NotContains(t, string(jsonBody), "runtime")
	require.NotContains(t, string(jsonBody), `"id"`)
	yamlBody, err := yaml.Marshal(nodes[0])
	require.NoError(t, err)
	require.NotContains(t, string(yamlBody), "runtime")
}

func TestRuntimeIDUsesOneWireNameOutsideNodeIR(t *testing.T) {
	for _, value := range []any{
		domain.SubscriptionPreviewNodeDiff{RuntimeID: "runtime-1", Status: "unchanged"},
		domain.NodeProbeResult{RuntimeID: "runtime-1"},
		domain.ValidationIssue{RuntimeID: "runtime-1"},
	} {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		require.Contains(t, string(body), `"runtime_id":"runtime-1"`)
		require.NotContains(t, string(body), "node_id")
	}
}

func TestNodeConnectionKeyClassifiesEveryExportedNodeField(t *testing.T) {
	connectionFields := map[string]bool{
		"Type": true, "Server": true, "Port": true, "Network": true,
		"Username": true, "Password": true, "UUID": true, "Cipher": true,
		"AlterID": true, "Flow": true, "Encryption": true, "Token": true,
		"PacketEncoding": true, "Plugin": true, "PluginOptions": true,
		"ShadowsocksR": true, "Snell": true, "AnyTLS": true, "Headers": true,
		"Path": true, "TLS": true, "Dialer": true, "Transport": true,
		"Multiplex": true, "UDPOverTCP": true, "Hysteria": true, "TUIC": true,
		"Mieru": true, "WireGuard": true,
	}
	nonConnectionFields := map[string]bool{
		"Name": true, "Tags": true, "Meta": true, "Raw": true,
		"Lossy": true, "Warnings": true, "SourceFormat": true,
	}
	typeOfNode := reflect.TypeOf(domain.NodeIR{})
	exportedCount := 0
	for index := range typeOfNode.NumField() {
		field := typeOfNode.Field(index)
		if field.PkgPath != "" {
			continue
		}
		exportedCount++
		require.NotEqual(t, connectionFields[field.Name], nonConnectionFields[field.Name], "field %s must have exactly one identity classification", field.Name)
	}
	require.Equal(t, exportedCount, len(connectionFields)+len(nonConnectionFields))
}

func TestAssignNodeRuntimeIDsPreservesUniqueAndReplacesCopies(t *testing.T) {
	nodes := []domain.NodeIR{{Name: "one"}}
	domain.AssignNodeRuntimeIDs(nodes)
	original := domain.NodeRuntimeID(nodes[0])
	nodes = append(nodes, nodes[0])
	domain.AssignNodeRuntimeIDs(nodes)
	require.Equal(t, original, domain.NodeRuntimeID(nodes[0]))
	require.NotEqual(t, original, domain.NodeRuntimeID(nodes[1]))
}

func TestNodeConnectionKeySeparatesConnectionFromPresentation(t *testing.T) {
	base := domain.NodeIR{
		Name: "one", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111",
		TLS:  &domain.TLSOptions{Enabled: true, ServerName: "cdn.example.com"},
	}
	baseKey, err := domain.NodeConnectionKey(base)
	require.NoError(t, err)
	require.Len(t, baseKey, 64)
	require.NotContains(t, baseKey, ":")

	presentation := base
	presentation.Name = "renamed"
	presentation.Tags = []string{"fast"}
	presentation.Meta = map[string]string{"probe.alive": "true"}
	presentation.SourceFormat = "mihomo"
	presentationKey, err := domain.NodeConnectionKey(presentation)
	require.NoError(t, err)
	require.Equal(t, baseKey, presentationKey)

	changed := base
	changed.TLS = &domain.TLSOptions{Enabled: true, ServerName: "other.example.com"}
	changedKey, err := domain.NodeConnectionKey(changed)
	require.NoError(t, err)
	require.NotEqual(t, baseKey, changedKey)
}

func TestNodeConnectionKeyIsStableWithNestedMaps(t *testing.T) {
	node := domain.NodeIR{Type: domain.NodeTypeShadowsocks, Server: "example.com", Port: 443,
		PluginOptions: map[string]any{"mode": "tls", "options": map[string]any{"host": "cdn.example.com", "path": "/proxy"}},
		Headers:       map[string]string{"X-One": "one", "X-Two": "two"},
	}
	want, err := domain.NodeConnectionKey(node)
	require.NoError(t, err)
	for range 32 {
		got, err := domain.NodeConnectionKey(node)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestNodeJSONPreservesExplicitEmptyDownloadOverrides(t *testing.T) {
	options := domain.XHTTPDownloadSettings{Path: new(""), Host: new(""), Port: new(uint16(0))}
	body, err := json.Marshal(options)
	require.NoError(t, err)
	require.JSONEq(t, `{"path":"","host":"","port":0}`, string(body))
	var decoded domain.XHTTPDownloadSettings
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, options, decoded)
}
