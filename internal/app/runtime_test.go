package app

import (
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

func TestValidateRejectsUnsupportedLogLevel(t *testing.T) {
	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: "127.0.0.1:1137"},
		Log:     LogConfig{Level: "trace"},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported log level")
}

func TestValidateRejectsPublicMCPPathOverride(t *testing.T) {
	err := Validate(Config{
		DataDir: t.TempDir(),
		HTTP:    HTTPConfig{Listen: "127.0.0.1:1137"},
		MCP:     MCPConfig{Path: "/convert"},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "conflicts with public route")
}
