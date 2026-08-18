package app

import (
	"context"
	"errors"
	"os"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func ResolveSettings(ctx context.Context, cfg Config, repository *store.SettingsStore) (Config, error) {
	startupToken := cfg.HTTP.Token
	stored, err := repository.Get(ctx)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
		stored = projectsettings.Default()
	} else {
		stored, err = projectsettings.Normalize(stored)
		if err != nil {
			return Config{}, err
		}
	}

	effective := stored
	applyStartupOverrides(&effective, cfg, cfg.OverrideSources)
	normalized, err := projectsettings.Normalize(effective)
	if err != nil {
		return Config{}, err
	}

	cfg.StoredSettings = stored
	cfg.EffectiveSettings = normalized
	cfg.HTTP = HTTPConfig{
		Listen: normalized.HTTP.Listen,
		Token:  startupToken,
	}
	cfg.MCP = MCPConfig{
		Path:           normalized.MCP.Path,
		MaxOutputBytes: normalized.MCP.MaxOutputBytes,
	}
	cfg.Log = LogConfig{Level: normalized.Log.Level}
	if cfg.OverrideSources == nil {
		cfg.OverrideSources = map[string]string{}
	}
	return cfg, nil
}

func applyStartupOverrides(value *domain.Settings, cfg Config, sources map[string]string) {
	if _, ok := sources["http.listen"]; ok {
		value.HTTP.Listen = cfg.HTTP.Listen
	}
	if _, ok := sources["mcp.path"]; ok {
		value.MCP.Path = cfg.MCP.Path
	}
	if _, ok := sources["mcp.max_output_bytes"]; ok {
		value.MCP.MaxOutputBytes = cfg.MCP.MaxOutputBytes
	}
	if _, ok := sources["log.level"]; ok {
		value.Log.Level = cfg.Log.Level
	}
}

func withProgrammaticOverrideSources(cfg Config) Config {
	if cfg.OverrideSources == nil {
		cfg.OverrideSources = map[string]string{}
	}
	mark := func(path string, nonDefault bool) {
		if nonDefault {
			if _, exists := cfg.OverrideSources[path]; !exists {
				cfg.OverrideSources[path] = "programmatic"
			}
		}
	}
	mark("http.listen", cfg.HTTP.Listen != "")
	mark("mcp.path", cfg.MCP.Path != "")
	mark("mcp.max_output_bytes", cfg.MCP.MaxOutputBytes != 0)
	mark("log.level", cfg.Log.Level != "")
	return cfg
}
