package script

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeSubsetIdentityIgnoresMapInsertionOrder(t *testing.T) {
	node := ScriptNode{Name: "node", Type: "vless", TLS: map[string]any{
		"enabled": true, "server_name": "example.com", "reality": map[string]any{"enabled": true, "public_key": "key"},
	}}
	allowed, err := scriptProbeNodeCounts([]ScriptNode{node})
	require.NoError(t, err)
	for range 32 {
		ok, err := scriptProbeNodesAllowed(allowed, []ScriptNode{node})
		require.NoError(t, err)
		require.True(t, ok)
	}
	node.TLS["server_name"] = "changed.example.com"
	ok, err := scriptProbeNodesAllowed(allowed, []ScriptNode{node})
	require.NoError(t, err)
	require.False(t, ok)
}

func TestProbeSubsetRejectsUnencodableIdentities(t *testing.T) {
	valid := ScriptNode{Name: "node", Type: "vless"}
	allowed, err := scriptProbeNodeCounts([]ScriptNode{valid})
	require.NoError(t, err)
	for name, invalid := range map[string]ScriptNode{
		"invalid UTF-8":    {Name: "invalid\xff", Type: "vless"},
		"nonfinite option": {Name: "node", Type: "vless", Ext: map[string]any{"value": math.NaN()}},
	} {
		t.Run(name, func(t *testing.T) {
			counts, err := scriptProbeNodeCounts([]ScriptNode{valid, invalid})
			require.ErrorContains(t, err, "input node 1: encode node identity")
			require.Nil(t, counts)
			ok, err := scriptProbeNodesAllowed(allowed, []ScriptNode{invalid})
			require.ErrorContains(t, err, "probe node 0: encode node identity")
			require.False(t, ok)
			api := &scriptAPI{}
			err = api.begin(t.Context(), ScriptEnvelope{Nodes: []ScriptNode{invalid}})
			require.Error(t, err)
			require.Nil(t, api.probeNodeCounts)
			require.Nil(t, api.ctx)
		})
	}
}
