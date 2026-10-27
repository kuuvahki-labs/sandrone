package service_test

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func utf8IdentityProcessor(t *testing.T) []domain.ProcessorSpec {
	t.Helper()
	return []domain.ProcessorSpec{{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{
		"content": `function main(input) { input.nodes.forEach(n => { n.tags = ["script-ran"]; }); return input; }`,
	})}}
}

func TestParseURIDisplayNameUTF8BeforeScriptAndReport(t *testing.T) {
	content := "ss://aes-128-gcm:fixture@example.com:8388?extra=value#%FF\n" +
		"ss://aes-128-gcm:fixture@example.org:8388#healthy"
	result, err := service.New().Parse(context.Background(), domain.ParseRequest{
		Format: "uri-list", Content: []byte(content), Processors: utf8IdentityProcessor(t),
	})
	require.NoError(t, err)
	require.Len(t, result.Nodes, 2)
	require.Equal(t, "\uFFFD", result.Nodes[0].Name)
	for _, node := range result.Nodes {
		require.Equal(t, []string{"script-ran"}, node.Tags)
	}
	require.NotEmpty(t, result.Report.Warnings)
	require.Equal(t, "\uFFFD", result.Report.Warnings[0].Node)
	require.Equal(t, "\uFFFD", result.Report.Warnings[0].NodeContext.Name)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.True(t, utf8.Valid(encoded))
}

func TestParseRejectsInvalidUTF8ConnectionFieldsBeforeScript(t *testing.T) {
	for _, bad := range []string{
		"ss://aes-128-gcm:private-secret%FF@example.com:8388#bad",
		"vless://private-identity%FF@example.com:443#bad",
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&path=private-path%FF#bad",
		"ss://aes-128-gcm:fixture@example.com:8388?plugin=obfs-local%3Bobfs-host%3Dprivate-host%FF#bad",
	} {
		t.Run(strings.SplitN(bad, "://", 2)[0], func(t *testing.T) {
			result, err := service.New().Parse(context.Background(), domain.ParseRequest{
				Format: "uri-list", Content: []byte(bad + "\nss://aes-128-gcm:fixture@example.org:8388#healthy"),
				Processors: utf8IdentityProcessor(t),
			})
			require.NoError(t, err)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "healthy", result.Nodes[0].Name)
			require.Equal(t, []string{"script-ran"}, result.Nodes[0].Tags)
			require.Len(t, result.Report.Warnings, 1)
			require.Equal(t, "node_validation_dropped", result.Report.Warnings[0].Code)
			require.Equal(t, "encoding", result.Report.Warnings[0].Field)
			encoded, err := json.Marshal(result.Report)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "private-")
		})
	}
}

func TestParseValidJSONRejectsDecodedBinaryHysteriaAuth(t *testing.T) {
	encodedAuth := base64.StdEncoding.EncodeToString([]byte("private-auth\xff"))
	content := fmt.Sprintf(`{"outbounds":[{"type":"hysteria","tag":"bad","server":"example.com","server_port":443,"auth":%q,"up_mbps":10,"down_mbps":20,"tls":{"enabled":true}},{"type":"shadowsocks","tag":"healthy","server":"example.org","server_port":8388,"method":"aes-128-gcm","password":"fixture"}]}`, encodedAuth)
	result, err := service.New().Parse(context.Background(), domain.ParseRequest{
		Format: "sing-box", Content: []byte(content), Processors: utf8IdentityProcessor(t),
	})
	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "healthy", result.Nodes[0].Name)
	require.Equal(t, []string{"script-ran"}, result.Nodes[0].Tags)
	require.Len(t, result.Report.Warnings, 1)
	require.Equal(t, "encoding", result.Report.Warnings[0].Field)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-auth")
	require.NotContains(t, string(encoded), encodedAuth)
}

func TestStructuredInputsStillRejectInvalidUTF8(t *testing.T) {
	for format, content := range map[string]string{
		"json-nodes": `[{"name":"bad` + "\xff" + `","type":"ss","server":"example.com","port":8388}]`,
		"sing-box":   `{"outbounds":[{"tag":"bad` + "\xff" + `","type":"shadowsocks","server":"example.com","server_port":8388,"method":"aes-128-gcm","password":"fixture"}]}`,
		"mihomo":     "proxies:\n  - name: bad\xff\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: aes-128-gcm\n    password: fixture\n",
	} {
		t.Run(format, func(t *testing.T) {
			_, err := service.New().Parse(context.Background(), domain.ParseRequest{Format: format, Content: []byte(content)})
			require.Error(t, err)
			require.False(t, domain.IsCode(err, domain.CodeNodeValidationFailed), "decoder must reject before semantic normalization")
		})
	}
}
