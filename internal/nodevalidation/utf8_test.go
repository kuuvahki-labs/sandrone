package nodevalidation_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

func TestValidateNodeEncodingBoundary(t *testing.T) {
	for name, mutate := range map[string]func(*domain.NodeIR){
		"display name":     func(n *domain.NodeIR) { n.Name = "bad\xff" },
		"credential":       func(n *domain.NodeIR) { n.Password = "private\xff" },
		"nested transport": func(n *domain.NodeIR) { n.Transport = &domain.TransportOptions{Type: "ws", Path: "private\xff"} },
		"map key":          func(n *domain.NodeIR) { n.Headers = map[string]string{"private\xff": "value"} },
		"map value":        func(n *domain.NodeIR) { n.PluginOptions = map[string]any{"raw": []any{"private\xff"}} },
		"raw JSON":         func(n *domain.NodeIR) { n.Raw = map[string]jsontext.Value{"private": []byte(`"\xff"`)} },
	} {
		t.Run(name, func(t *testing.T) {
			node := domain.NodeIR{Name: "safe", Type: domain.NodeTypeShadowsocks, Server: "example.com", Port: 443, Cipher: "aes-128-gcm", Password: "fixture"}
			mutate(&node)
			result := nodevalidation.Validate([]domain.NodeIR{node}, nodevalidation.StageNormalized, "")
			require.Empty(t, result.Nodes)
			require.Len(t, result.Issues, 1)
			require.Equal(t, "encoding", result.Issues[0].Field)
			if name == "display name" {
				require.Equal(t, "bad\uFFFD", result.Issues[0].NodeName)
			} else {
				require.Equal(t, "safe", result.Issues[0].NodeName)
			}
			encoded, err := json.Marshal(result.Issues)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "private")
		})
	}
}
