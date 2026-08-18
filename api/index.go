// Package handler exposes Sandrone as a Vercel Go Function.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/webui"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

type runtimeFactory func(context.Context, app.Config, *slog.Logger, ...app.RuntimeOption) (*app.Runtime, error)

var productionHandler = sync.OnceValues(newProductionHandler)

func Handler(w http.ResponseWriter, r *http.Request) {
	handler, err := productionHandler()
	if err != nil {
		slog.Default().Error("Vercel handler initialization failed", "category", "runtime_initialization_failed")
		writeInitializationError(w)
		return
	}
	handler.ServeHTTP(w, r)
}

func newProductionHandler() (http.Handler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return buildHandler(ctx, os.Getenv, app.NewRuntimeContext)
}

func buildHandler(ctx context.Context, getenv func(string) string, factory runtimeFactory) (http.Handler, error) {
	token := strings.TrimSpace(getenv("SANDRONE_TOKEN"))
	if token == "" {
		return nil, errors.New("SANDRONE_TOKEN is required")
	}
	if factory == nil {
		return nil, errors.New("runtime factory is required")
	}
	env := storageEnvironment(getenv)
	storageConfig, err := app.StorageConfigFromEnv(env)
	if err != nil {
		return nil, err
	}
	if storageConfig.Backend != app.StorageS3 {
		return nil, errors.New("vercel runtime requires SANDRONE_STORAGE_BACKEND=s3")
	}
	maxOutput, err := environmentInt(getenv("SANDRONE_MCP_MAX_OUTPUT_BYTES"), 1<<20)
	if err != nil || maxOutput < 0 {
		return nil, errors.New("SANDRONE_MCP_MAX_OUTPUT_BYTES must be a non-negative integer")
	}
	mcpPath := strings.TrimSpace(getenv("SANDRONE_MCP_PATH"))
	if mcpPath == "" {
		mcpPath = app.DefaultMCPPath
	}
	logLevel := strings.TrimSpace(getenv("SANDRONE_LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}
	logger, err := app.NewLogger(app.LogConfig{Level: logLevel}, os.Stderr)
	if err != nil {
		return nil, err
	}
	runtime, err := factory(ctx, app.Config{
		DataDir: app.DefaultDataDir,
		Storage: storageConfig,
		HTTP: app.HTTPConfig{
			Listen: app.DefaultListen,
			Token:  token,
		},
		MCP: app.MCPConfig{
			Path:           mcpPath,
			MaxOutputBytes: maxOutput,
		},
		Log: app.LogConfig{Level: logLevel},
	}, logger, app.WithProbeEngine(probe.NewDisabled()), app.WithSchedulerEnabled(false))
	if err != nil {
		return nil, err
	}
	mcpServer := mcpapi.New(runtime)
	webHandler := webui.Handler(webui.WithReservedPrefixes("/v1", runtime.Config.MCP.Path, "/s"))
	return httpapi.New(
		runtime,
		httpapi.WithMCP(runtime.Config.MCP.Path, mcpServer.Handler()),
		httpapi.WithWebUI(webHandler),
	).Handler(), nil
}

func storageEnvironment(getenv func(string) string) map[string]string {
	keys := []string{
		app.EnvStorageBackend,
		app.EnvS3Endpoint,
		app.EnvS3Region,
		app.EnvS3Bucket,
		app.EnvS3Prefix,
		app.EnvS3ForcePathStyle,
		app.EnvS3AccessKeyID,
		app.EnvS3SecretAccessKey,
		app.EnvS3SessionToken,
	}
	env := make(map[string]string, len(keys))
	for _, key := range keys {
		env[key] = getenv(key)
	}
	return env
}

func environmentInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return strconv.Atoi(strings.TrimSpace(raw))
}

func writeInitializationError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "runtime_initialization_failed",
			"message": "application runtime is unavailable",
		},
	})
}
