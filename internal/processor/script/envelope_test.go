package script

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeToScriptRoundtrip(t *testing.T) {
	in := domain.NodeIR{
		Name:   "n1",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		TLS:    &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
		Transport: &domain.TransportOptions{
			Type: "websocket",
			Path: "/ws",
		},
	}
	domain.SetNodeLineage(&in, "origin-1")
	s, err := nodeToScript(in)
	require.NoError(t, err)
	require.Equal(t, in.Name, s.Name)
	require.Equal(t, string(in.Type), s.Type)

	out, warnings, err := scriptToNode(s)
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Equal(t, in.Name, out.Name)
	require.Equal(t, in.UUID, out.UUID)
	require.NotNil(t, out.TLS)
	require.True(t, out.TLS.Enabled)
	require.Equal(t, "origin-1", domain.NodeLineage(out))
}

func TestNodeToScriptRoundtripPreservesAdvancedProtocolOptions(t *testing.T) {
	reuse := true
	in := domain.NodeIR{
		Name: "advanced", Type: domain.NodeTypeSnell, Server: "example.com", Port: 443, Password: "secret",
		Snell:  &domain.SnellOptions{Version: 5, Reuse: &reuse, ClientFingerprint: "chrome", ShadowTLS: &domain.ShadowTLSOptions{Password: "shadow", Host: "cdn.example.com", Version: 3}},
		AnyTLS: &domain.AnyTLSOptions{IdleSessionCheckInterval: "30s", MinIdleSession: 1},
		Mieru:  &domain.MieruOptions{Transport: "TCP"},
		Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			Mode: "packet-up", ReuseSettings: &domain.XHTTPReuseSettings{MaxConcurrency: "8-16"},
		}},
	}

	scriptNode, err := nodeToScript(in)
	require.NoError(t, err)
	out, warnings, err := scriptToNode(scriptNode)
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Equal(t, in.Snell, out.Snell)
	require.Equal(t, in.AnyTLS, out.AnyTLS)
	require.Equal(t, in.Mieru, out.Mieru)
	require.Equal(t, in.Transport, out.Transport)
}

func TestScriptToNodeExtField(t *testing.T) {
	s := ScriptNode{
		Name: "x",
		Type: string(domain.NodeTypeShadowsocks),
		Ext:  map[string]any{"custom": "value"},
	}
	node, warnings, err := scriptToNode(s)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Equal(t, "script_ext_field", warnings[0].Code)
	require.Contains(t, node.Raw, "script.ext.custom")
}

func TestScriptToNodeRejectsMissingType(t *testing.T) {
	_, _, err := scriptToNode(ScriptNode{Name: "x"})
	require.ErrorContains(t, err, "without type")
}

func TestScriptToNodeRejectsMissingName(t *testing.T) {
	_, _, err := scriptToNode(ScriptNode{Type: string(domain.NodeTypeShadowsocks)})
	require.ErrorContains(t, err, "without name")
}

func TestScriptToNodesReportsIndex(t *testing.T) {
	_, _, err := scriptToNodes([]ScriptNode{
		{Name: "ok", Type: string(domain.NodeTypeShadowsocks)},
		{Name: "bad"},
	})
	require.ErrorContains(t, err, "node 1")
}

func TestFileEnvelope(t *testing.T) {
	baseFile := domain.FileDocument{
		Name:    "cfg.yaml",
		Kind:    "mihomo",
		Content: []byte("a: 1"),
		Meta:    map[string]string{"k": "v"},
	}
	sf := fileToScript(baseFile)
	require.Equal(t, "cfg.yaml", sf.Name)
	restored := scriptToFile(sf, baseFile)
	require.Equal(t, baseFile.Content, restored.Content)
	require.Equal(t, "v", restored.Meta["k"])
}

func TestScriptToFileFallbacks(t *testing.T) {
	baseFile := domain.FileDocument{
		Name:      "base.yaml",
		Kind:      "yaml",
		MediaType: "application/yaml",
		Encoding:  "utf-8",
		Content:   []byte("a: 1"),
		Meta:      map[string]string{"keep": "yes"},
		Warnings:  []domain.Warning{{Code: "existing"}},
	}
	require.Equal(t, baseFile, scriptToFile(nil, baseFile))

	updatedFile := scriptToFile(&ScriptFile{
		Name:      "updated.json",
		Kind:      "json",
		MediaType: "application/json",
		Encoding:  "base64",
		Content:   `{"ok":true}`,
		Meta:      map[string]string{"new": "yes"},
		Warnings:  []domain.Warning{{Code: "script"}},
	}, baseFile)
	require.Equal(t, "updated.json", updatedFile.Name)
	require.Equal(t, "json", updatedFile.Kind)
	require.Equal(t, "application/json", updatedFile.MediaType)
	require.Equal(t, "base64", updatedFile.Encoding)
	require.Equal(t, []byte(`{"ok":true}`), updatedFile.Content)
	require.Equal(t, map[string]string{"new": "yes"}, updatedFile.Meta)
	require.Equal(t, []domain.Warning{{Code: "script"}}, updatedFile.Warnings)
}

func TestPartsToScript(t *testing.T) {
	parts, err := partsToScript([]domain.FilePart{{
		Name: "nodes",
		Role: "nodes",
		Kind: "yaml",
		Nodes: []domain.NodeIR{{
			Name: "a", Type: domain.NodeTypeShadowsocks, Server: "x", Port: 1,
		}},
	}})
	require.NoError(t, err)
	require.Len(t, parts, 1)
	require.Len(t, parts[0].Nodes, 1)
}

func TestCloneStringMapNil(t *testing.T) {
	require.Nil(t, cloneStringMap(nil))
	m := cloneStringMap(map[string]string{"a": "b"})
	require.Equal(t, "b", m["a"])
}
