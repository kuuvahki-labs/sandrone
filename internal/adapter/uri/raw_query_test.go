package uri_test

import (
	"context"
	"encoding/json/v2"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
)

func TestUnknownQueryRemainsValidJSON(t *testing.T) {
	for _, base := range []string{
		"vless://00000000-0000-0000-0000-000000000001@example.com:443",
		"ss://aes-128-gcm:secret@example.com:8388",
	} {
		// Include the Outline TLS ClientHello prefix that triggered the real source failure.
		for _, value := range []string{"\u0016\u0003\u0001\u0000\u00a8\u0001\u0001", "\x00\x01\a\b\t\n\v\f\r\x1f\x7f", "引号\"、反斜线\\、emoji 🌍"} {
			t.Run(base+"/"+url.QueryEscape(value), func(t *testing.T) {
				nodes, _, err := uri.NewParser().Parse(context.Background(), []byte(base+"?prefix="+url.QueryEscape(value)))
				require.NoError(t, err)
				require.Len(t, nodes, 1)
				_, err = json.Marshal(nodes)
				require.NoError(t, err, "unknown query metadata must not make the node pool invalid JSON")
				var decoded string
				require.NoError(t, json.Unmarshal(nodes[0].Raw["uri.query.prefix"], &decoded))
				require.Equal(t, value, decoded)
			})
		}
	}
}
