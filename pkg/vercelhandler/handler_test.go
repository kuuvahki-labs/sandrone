package vercelhandler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/envconfig"
)

func vercelTestEnv() map[string]string {
	return map[string]string{
		envconfig.Token:             "test-token",
		envconfig.StorageBackend:    "s3",
		envconfig.S3Endpoint:        "https://account.example.invalid",
		envconfig.S3Region:          "auto",
		envconfig.S3Bucket:          "bucket",
		envconfig.S3Prefix:          "preview/",
		envconfig.S3AccessKeyID:     "access-marker",
		envconfig.S3SecretAccessKey: "secret-marker",
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
	delete(env, envconfig.Token)
	_, err := buildHandler(context.Background(), envLookup(env), nil)
	require.ErrorContains(t, err, envconfig.Token)
}

func TestBuildHandlerRequiresS3Backend(t *testing.T) {
	env := vercelTestEnv()
	env[envconfig.StorageBackend] = "filesystem"
	_, err := buildHandler(context.Background(), envLookup(env), func(context.Context, app.Config, *slog.Logger, ...app.RuntimeOption) (*app.Runtime, error) {
		return nil, errors.New("must not run")
	})
	require.ErrorContains(t, err, "requires "+envconfig.StorageBackend+"=s3")
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

func TestInitializationErrorResponseIsSanitized(t *testing.T) {
	response := httptest.NewRecorder()
	writeInitializationError(response)
	require.NotContains(t, response.Body.String(), "secret-marker")
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), "runtime_initialization_failed")
}
