package store

import (
	"context"
	"errors"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
)

const SettingsKey = "settings.json"

type SettingsStore struct {
	coordinator Coordinator
}

func NewSettingsStore(coordinator Coordinator) *SettingsStore {
	return &SettingsStore{coordinator: coordinator}
}

func (s *SettingsStore) Get(ctx context.Context) (domain.Settings, error) {
	var value domain.Settings
	var rewrite bool
	err := s.coordinator.View(ctx, func(raw Store) error {
		body, err := raw.Read(ctx, SettingsKey)
		if err != nil {
			return err
		}
		value, rewrite, err = projectsettings.DecodeStored(body)
		return err
	})
	if err != nil {
		return domain.Settings{}, err
	}
	if rewrite {
		if err := s.Put(ctx, value); err != nil {
			return domain.Settings{}, err
		}
	}
	return value, nil
}

func (s *SettingsStore) Put(ctx context.Context, value domain.Settings) error {
	body, err := marshalStoreJSON(value)
	if err != nil {
		return err
	}
	return s.coordinator.Update(ctx, func(raw Store) error {
		writer, ok := raw.(AtomicWriter)
		if !ok {
			return errors.New("settings store requires atomic file writes")
		}
		return writer.WriteAtomic(ctx, SettingsKey, body, 0o600)
	})
}
