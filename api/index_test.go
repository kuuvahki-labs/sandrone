package handler

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntrypointDoesNotImportInternalPackages(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "index.go", nil, parser.ImportsOnly)
	require.NoError(t, err)

	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)
		require.NotContains(t, path, "/internal/", "Vercel compiles api/index.go outside the Sandrone module")
		imports = append(imports, path)
	}
	require.Contains(t, imports, "github.com/kuuvahki-labs/sandrone/pkg/vercelhandler")
}

func TestVercelConfigContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "vercel.json"))
	require.NoError(t, err)
	var cfg struct {
		Framework json.RawMessage `json:"framework"`
		Build     struct {
			Env map[string]string `json:"env"`
		} `json:"build"`
		Functions map[string]struct {
			MaxDuration int `json:"maxDuration"`
		} `json:"functions"`
		Rewrites []struct {
			Source      string `json:"source"`
			Destination string `json:"destination"`
		} `json:"rewrites"`
	}
	require.NoError(t, json.Unmarshal(body, &cfg))
	require.JSONEq(t, "null", string(cfg.Framework))
	require.Equal(t, "-ldflags '-s -w'", cfg.Build.Env["GO_BUILD_FLAGS"])
	require.Len(t, cfg.Functions, 1)
	require.Equal(t, 60, cfg.Functions["api/index.go"].MaxDuration)
	require.Equal(t, "/(.*)", cfg.Rewrites[0].Source)
	require.Equal(t, "/api/index.go", cfg.Rewrites[0].Destination)
	for _, forbidden := range []string{"-tags", "probe_singbox", "with_quic", "with_wireguard", "with_utls"} {
		require.False(t, strings.Contains(cfg.Build.Env["GO_BUILD_FLAGS"], forbidden))
	}
}
