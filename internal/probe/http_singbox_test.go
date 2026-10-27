//go:build probe_singbox

package probe

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPStartFailure(t *testing.T) {
	results, err := RequestNodeHTTP(context.Background(), []domain.NodeIR{{Name: "node"}}, domain.NodeHTTPOptions{URL: "https://example.com/", Method: "GET", TimeoutMS: 5000, Concurrency: 4, MaxResponseBytes: 65536}, Payload{Body: []byte(`{"outbounds":[{"type":"http","tag":"node","server":"127.0.0.1","server_port":80,"detour":"missing"}]}`)})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, results[0].Error, "detour")
}

func TestNodeHTTPSingBoxConstructionFailuresAreIsolated(t *testing.T) {
	for _, bad := range []map[string]any{
		{"type": "tuic", "uuid": "11111111-1111-1111-1111-111111111111", "password": "fixture", "congestion_control": "bbr_tag=M002-260618", "tls": map[string]any{"enabled": true}},
		{"type": "shadowsocks", "method": "2022-blake3-aes-128-gcm", "password": "not-a-base64-key"},
		{"type": "http", "connect_timeout": "invalid-duration"},
		{"type": "unknown"},
	} {
		t.Run(bad["type"].(string), func(t *testing.T) {
			good, observations, count := httpBackendProxy(t, func(*http.Request) (int, string, http.Header) { return 200, "healthy", nil })
			good.Name = "healthy"
			bad["tag"], bad["server"], bad["server_port"] = "bad", "127.0.0.1", 9
			body, err := json.Marshal(map[string]any{"outbounds": []map[string]any{
				// Deliberately reverse payload order to verify association by tag.
				{"type": "http", "tag": good.Name, "server": good.Server, "server_port": good.Port}, bad,
			}})
			require.NoError(t, err)
			results, err := RequestNodeHTTP(t.Context(), []domain.NodeIR{{Name: "bad"}, good}, backendHTTPOptions("sing-box"), Payload{Body: body})
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.NotEmpty(t, results[0].Error)
			require.Equal(t, 0, results[0].Index)
			require.Empty(t, results[1].Error)
			require.Equal(t, "healthy", results[1].Body)
			require.Equal(t, 1, results[1].Index)
			require.EqualValues(t, 1, count.Load())
			require.Equal(t, "exit.invalid", (<-observations).host)
		})
	}
}

func TestNodeHTTPSingBoxInvalidPayloadIsFatal(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{`, `{}`, `{"outbounds":42}`, `{"outbounds":null}`, `{"outbounds":[null]}`,
		`{"outbounds":[{"type":"http","tag":"different","server":"127.0.0.1","server_port":9}]}`,
		`{"outbounds":[{"type":"http","tag":"node","server":"127.0.0.1","server_port":9},{"type":"http","tag":"node","server":"127.0.0.1","server_port":9}]}`,
		`{"outbounds":[],"inbounds":[]}`,
	} {
		_, err := RequestNodeHTTP(t.Context(), []domain.NodeIR{{Name: "node"}}, backendHTTPOptions("sing-box"), Payload{Body: []byte(body)})
		require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget), "%s: %v", body, err)
	}
}
