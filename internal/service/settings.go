package service

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
)

func (s *Service) GetSettings(context.Context) (domain.SettingsSnapshot, error) {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return settingsSnapshot(s.storedSettings, s.effectiveSettings, s.settingsOverrides), nil
}

func (s *Service) PutSettings(ctx context.Context, update domain.SettingsUpdate) (domain.SettingsSnapshot, error) {
	if s.settingsStore == nil {
		return domain.SettingsSnapshot{}, storeUnavailable()
	}

	s.settingsMu.Lock()
	next, err := projectsettings.ApplyUpdate(update)
	if err != nil {
		s.settingsMu.Unlock()
		return domain.SettingsSnapshot{}, err
	}
	if err := s.settingsStore.Put(ctx, next); err != nil {
		s.settingsMu.Unlock()
		return domain.SettingsSnapshot{}, err
	}
	s.storedSettings = next
	applyDynamicSettings(&s.effectiveSettings, next)
	s.applyRuntimeCapabilities(&s.effectiveSettings)
	snapshot := settingsSnapshot(s.storedSettings, s.effectiveSettings, s.settingsOverrides)
	s.settingsMu.Unlock()

	s.notifyScheduledRefreshSettingsChanged()
	s.logResource(ctx, "put", "settings", "project")
	return snapshot, nil
}

func (s *Service) ReloadSettings(ctx context.Context) error {
	if s.settingsStore == nil {
		return storeUnavailable()
	}
	next, err := s.settingsStore.Get(ctx)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next = projectsettings.Default()
	} else {
		next, err = projectsettings.Normalize(next)
		if err != nil {
			return err
		}
	}

	s.settingsMu.Lock()
	s.storedSettings = next
	applyDynamicSettings(&s.effectiveSettings, next)
	s.applyRuntimeCapabilities(&s.effectiveSettings)
	s.settingsMu.Unlock()
	s.notifyScheduledRefreshSettingsChanged()
	return nil
}

func (s *Service) currentSettings() domain.Settings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.effectiveSettings
}

func applyDynamicSettings(effective *domain.Settings, stored domain.Settings) {
	effective.SchemaVersion = stored.SchemaVersion
	effective.RemoteDefaults = stored.RemoteDefaults
	effective.ProbeDefaults = stored.ProbeDefaults
	effective.ScriptDefaults = stored.ScriptDefaults
	effective.SpecifyCacheDefaults(stored.CacheDefaults)
	effective.Appearance = stored.Appearance
	effective.Subscriptions = stored.Subscriptions
	effective.ScheduledRefresh = stored.ScheduledRefresh
}

func (s *Service) applyRuntimeCapabilities(effective *domain.Settings) {
	if !s.schedulerEnabled {
		effective.ScheduledRefresh.Enabled = false
	}
}

func settingsSnapshot(stored, effective domain.Settings, overrides map[string]string) domain.SettingsSnapshot {
	return domain.SettingsSnapshot{
		Settings:        projectsettings.View(stored),
		Effective:       projectsettings.View(effective),
		Overrides:       cloneStringMap(overrides),
		RestartRequired: restartRequiredPaths(stored, effective, overrides),
	}
}

func restartRequiredPaths(stored, effective domain.Settings, overrides map[string]string) []string {
	paths := []string{}
	add := func(path string, changed bool) {
		if changed {
			if _, overridden := overrides[path]; !overridden {
				paths = append(paths, path)
			}
		}
	}
	add("http.listen", stored.HTTP.Listen != effective.HTTP.Listen)
	add("mcp.path", stored.MCP.Path != effective.MCP.Path)
	add("mcp.max_output_bytes", stored.MCP.MaxOutputBytes != effective.MCP.MaxOutputBytes)
	add("log.level", stored.Log.Level != effective.Log.Level)
	sort.Strings(paths)
	return paths
}
