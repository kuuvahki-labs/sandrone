package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
)

func vercelTestEnv() map[string]string {
	return map[string]string{
		"SANDRONE_TOKEN":         "test-token",
		app.EnvStorageBackend:    "s3",
		app.EnvS3Endpoint:        "https://account.example.invalid",
		app.EnvS3Region:          "auto",
		app.EnvS3Bucket:          "bucket",
		app.EnvS3Prefix:          "preview/",
		app.EnvS3AccessKeyID:     "access-marker",
		app.EnvS3SecretAccessKey: "secret-marker",
	}
}

func envLookup(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func filesystemRuntimeFactory(t *testing.T, captured *app.Config) runtimeFactory {
	t.Helper()
	dataDir := t.TempDir()
	return func(ctx context.Context, cfg app.Config, logger *slog.Logger, opts ...app.RuntimeOption) (*app.Runtime, error) {
		*captured = cfg
		cfg.DataDir = dataDir
		cfg.Storage = app.StorageConfig{Backend: app.StorageFilesystem}
		return app.NewRuntimeContext(ctx, cfg, logger, opts...)
	}
}

func TestBuildHandlerRequiresToken(t *testing.T) {
	env := vercelTestEnv()
	delete(env, "SANDRONE_TOKEN")
	_, err := buildHandler(context.Background(), envLookup(env), nil)
	require.ErrorContains(t, err, "SANDRONE_TOKEN")
}

func TestBuildHandlerRequiresS3Backend(t *testing.T) {
	env := vercelTestEnv()
	env[app.EnvStorageBackend] = "filesystem"
	_, err := buildHandler(context.Background(), envLookup(env), func(context.Context, app.Config, *slog.Logger, ...app.RuntimeOption) (*app.Runtime, error) {
		return nil, errors.New("must not run")
	})
	require.ErrorContains(t, err, "requires SANDRONE_STORAGE_BACKEND=s3")
}

func TestBuildHandlerServesVersionAndDisabledCapabilities(t *testing.T) {
	var captured app.Config
	handler, err := buildHandler(context.Background(), envLookup(vercelTestEnv()), filesystemRuntimeFactory(t, &captured))
	require.NoError(t, err)
	require.Equal(t, app.StorageS3, captured.Storage.Backend)

	versionRequest := httptest.NewRequest(http.MethodGet, "/version", nil)
	versionResponse := httptest.NewRecorder()
	handler.ServeHTTP(versionResponse, versionRequest)
	require.Equal(t, http.StatusOK, versionResponse.Code)

	request := httptest.NewRequest(http.MethodGet, "/v1/capabilities/ui", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Features []struct {
			Key     string `json:"key"`
			Enabled bool   `json:"enabled"`
		} `json:"features"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	features := map[string]bool{}
	for _, feature := range body.Features {
		features[feature.Key] = feature.Enabled
	}
	for _, key := range []string{"probe.enabled", "core.mihomo", "core.sing_box", "scheduler.enabled"} {
		require.Contains(t, features, key)
		require.False(t, features[key], key)
	}
}

func TestBuildHandlerProbeReturnsBackendUnavailable(t *testing.T) {
	var captured app.Config
	handler, err := buildHandler(context.Background(), envLookup(vercelTestEnv()), filesystemRuntimeFactory(t, &captured))
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/v1/probe", bytes.NewBufferString(`{}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "probe_backend_unavailable")
}

func TestInitializationErrorResponseIsSanitized(t *testing.T) {
	response := httptest.NewRecorder()
	writeInitializationError(response)
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "secret-marker")
	require.Contains(t, response.Body.String(), "runtime_initialization_failed")
}

func TestVercelConfigContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "vercel.json"))
	require.NoError(t, err)
	var cfg struct {
		Build struct {
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
	require.Equal(t, "-ldflags '-s -w'", cfg.Build.Env["GO_BUILD_FLAGS"])
	require.Len(t, cfg.Functions, 1)
	require.Equal(t, 60, cfg.Functions["api/index.go"].MaxDuration)
	require.Equal(t, "/(.*)", cfg.Rewrites[0].Source)
	require.Equal(t, "/api/index.go", cfg.Rewrites[0].Destination)
	for _, forbidden := range []string{"-tags", "probe_singbox", "with_quic", "with_wireguard", "with_utls"} {
		require.False(t, strings.Contains(cfg.Build.Env["GO_BUILD_FLAGS"], forbidden))
	}
}
