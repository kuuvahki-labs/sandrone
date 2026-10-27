package service_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestVMessCipherValidationAcrossInputs(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"uri-list", "mihomo", "sing-box", "json-nodes"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			input := func(cipher string) []byte {
				switch format {
				case "uri-list":
					body := fmt.Sprintf(`{"ps":"vmess","add":"proxy.example","port":443,"id":"11111111-1111-4111-8111-111111111111","scy":%s}`, cipher)
					return []byte("vmess://" + base64.StdEncoding.EncodeToString([]byte(body)))
				case "mihomo":
					return fmt.Appendf(nil, `{"proxies":[{"name":"vmess","type":"vmess","server":"proxy.example","port":443,"uuid":"11111111-1111-4111-8111-111111111111","cipher":%s}]}`, cipher)
				case "sing-box":
					return fmt.Appendf(nil, `{"outbounds":[{"tag":"vmess","type":"vmess","server":"proxy.example","server_port":443,"uuid":"11111111-1111-4111-8111-111111111111","security":%s}]}`, cipher)
				default:
					return fmt.Appendf(nil, `[{"name":"vmess","type":"vmess","server":"proxy.example","port":443,"uuid":"11111111-1111-4111-8111-111111111111","cipher":%s}]`, cipher)
				}
			}
			svc := service.New()
			for _, cipher := range []string{`"null"`, `"unknown"`} {
				_, err := svc.Parse(context.Background(), domain.ParseRequest{Format: format, Content: input(cipher)})
				require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed), "%s: %v", cipher, err)
			}
			for _, cipher := range []string{`null`, `""`, `"AES-128-GCM"`} {
				result, err := svc.Parse(context.Background(), domain.ParseRequest{Format: format, Content: input(cipher)})
				require.NoError(t, err, cipher)
				require.Len(t, result.Nodes, 1)
				if cipher == `"AES-128-GCM"` {
					require.Equal(t, "aes-128-gcm", result.Nodes[0].Cipher)
				} else {
					require.Contains(t, []string{"", "auto"}, result.Nodes[0].Cipher)
				}
			}
		})
	}
}

func TestVMessCipherValidationAfterScript(t *testing.T) {
	t.Parallel()
	_, err := service.New().Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "json-nodes", ToFormat: "json-nodes",
		Content: []byte(`[{"name":"vmess","type":"vmess","server":"proxy.example","port":443,"uuid":"11111111-1111-4111-8111-111111111111","cipher":"auto"}]`),
		ParseProcessors: []domain.ProcessorSpec{{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{
			"content": `function main(input) { input.nodes[0].cipher = "null"; return input; }`,
		})}},
	})
	require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed), "%v", err)
}
