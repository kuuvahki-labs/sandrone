package uri_test

import (
	"encoding/base64"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
)

func TestURINameUTF8AndUnknownWarnings(t *testing.T) {
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:fixture"))
	for _, input := range []string{
		"ss://" + userinfo + "@example.com:8388?extra=value#%FF",
		"ss://aes-128-gcm:fixture@example.com:8388?extra=value#%FF",
	} {
		nodes, source, err := uri.NewParser().Parse(t.Context(), []byte(input))
		require.NoError(t, err)
		require.Equal(t, "\uFFFD", nodes[0].Name)
		require.Equal(t, "fixture", nodes[0].Password)
		require.Equal(t, "\uFFFD", source.Warnings[0].Node)
		_, err = json.Marshal(source)
		require.NoError(t, err)
	}
}
