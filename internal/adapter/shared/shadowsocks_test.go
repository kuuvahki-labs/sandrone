package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func TestNormalizeShadowsocksCipherKnownAliasesOnly(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "legacy aead", in: "AEAD_AES_128_GCM", want: "aes-128-gcm"},
		{name: "upper canonical", in: "CHACHA20-IETF-POLY1305", want: "chacha20-ietf-poly1305"},
		{name: "unknown preserved", in: " Unknown_Cipher ", want: "Unknown_Cipher"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shared.NormalizeShadowsocksCipher(tc.in))
		})
	}
}

func TestNormalizeShadowsocksRCipherDummy(t *testing.T) {
	require.Equal(t, "none", shared.NormalizeShadowsocksRCipher(" DUMMY "))
}
