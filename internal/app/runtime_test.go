package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultsUsesBackendDefaultListen(t *testing.T) {
	cfg := Defaults(Config{})

	require.Equal(t, "127.0.0.1:1137", cfg.HTTP.Listen)
}

func TestValidateRequiresTokenForBarePortListen(t *testing.T) {
	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: ":1137"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires --token")
}

func TestValidateAllowsLocalhostListenWithoutToken(t *testing.T) {
	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: "127.0.0.1:1137"},
	})
	require.NoError(t, err)
}

func TestValidateRejectsWebUIStaticDirFile(t *testing.T) {
	staticFile := filepath.Join(t.TempDir(), "index.html")
	require.NoError(t, os.WriteFile(staticFile, []byte("<html></html>"), 0o644))

	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: "127.0.0.1:1137"},
		WebUI:   WebUIConfig{StaticDir: staticFile},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "Web UI static dir must be a directory")
}

func TestValidateRejectsUnsupportedLogLevel(t *testing.T) {
	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: "127.0.0.1:1137"},
		Log:     LogConfig{Level: "trace"},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported log level")
}
