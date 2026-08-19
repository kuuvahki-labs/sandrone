package uri_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
)

func TestShadowsocksCipherCanonicalizationMatchesURIAndMihomoInputs(t *testing.T) {
	for _, tc := range []struct {
		cipher string
		want   string
	}{
		{cipher: "AEAD_AES_128_GCM", want: "aes-128-gcm"},
		{cipher: "Vendor_Cipher", want: "Vendor_Cipher"},
	} {
		credentials := base64.RawURLEncoding.EncodeToString([]byte(tc.cipher + ":secret"))
		uriNodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte("ss://"+credentials+"@example.com:8388"))
		require.NoError(t, err)

		mihomoNodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte("proxies:\n  - name: ss\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: "+tc.cipher+"\n    password: secret\n"))
		require.NoError(t, err)

		require.Equal(t, tc.want, uriNodes[0].Cipher)
		require.Equal(t, uriNodes[0].Cipher, mihomoNodes[0].Cipher)
	}
}
