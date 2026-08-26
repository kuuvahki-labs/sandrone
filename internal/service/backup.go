package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	backuppkg "github.com/kuuvahki-labs/sandrone/internal/backup"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

// ExportBackup snapshots every non-cache Store file and encodes its raw key
// and bytes in the versioned backup ZIP contract.
func (s *Service) ExportBackup(ctx context.Context) (*domain.BackupExportResult, error) {
	if s.storeCoordinator == nil {
		return nil, errors.New("backup Store is not configured")
	}

	createdAt := s.now().UTC()
	var entries []backuppkg.Entry
	err := s.storeCoordinator.View(ctx, func(resourceStore store.Store) error {
		var err error
		entries, err = backuppkg.ReadEntries(ctx, resourceStore)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("export backup: %w", err)
	}

	body, err := backuppkg.Encode(entries, createdAt)
	if err != nil {
		return nil, fmt.Errorf("export backup: %w", err)
	}
	return &domain.BackupExportResult{
		Body:     body,
		Filename: "sandrone-backup-" + createdAt.Format("20060102T150405Z") + ".zip",
	}, nil
}

// RestoreBackup validates a complete backup before replacing the Store under
// its coordination lock. Cache bytes are intentionally neither imported nor
// restored during rollback.
func (s *Service) RestoreBackup(ctx context.Context, body []byte) error {
	files, err := backuppkg.Decode(body)
	if err != nil {
		return err
	}
	if s.storeCoordinator == nil {
		return domain.NewError(domain.CodeBackupRestoreFailed, "backup Store is not configured")
	}

	mutationCtx := context.WithoutCancel(ctx)
	var snapshot backuppkg.Snapshot
	err = s.storeCoordinator.Update(ctx, func(resourceStore store.Store) error {
		snapshot, err = backuppkg.Capture(ctx, resourceStore)
		if err != nil {
			return domain.WrapError(domain.CodeBackupRestoreFailed, "backup restore failed", err)
		}

		if err := snapshot.Replace(mutationCtx, resourceStore, files); err != nil {
			rollbackErr := snapshot.Restore(mutationCtx, resourceStore)
			if rollbackErr != nil {
				s.log(mutationCtx, slog.LevelError, "service backup restore rollback failed",
					"restore_cause", backuppkg.StoreOperation(err),
					"rollback_cause", backuppkg.StoreOperation(rollbackErr),
				)
				return domain.WrapError(
					domain.CodeBackupRestoreFailed,
					"backup restore and rollback failed",
					errors.Join(err, rollbackErr),
				)
			}
			return domain.WrapError(domain.CodeBackupRestoreFailed, "backup restore failed", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.ReloadSettings(mutationCtx); err != nil {
		rollbackErr := s.storeCoordinator.Update(mutationCtx, func(resourceStore store.Store) error {
			return snapshot.Restore(mutationCtx, resourceStore)
		})
		reloadErr := s.ReloadSettings(mutationCtx)
		if rollbackErr != nil || reloadErr != nil {
			s.log(mutationCtx, slog.LevelError, "service backup settings rollback failed",
				"restore_cause", "reload restored settings",
				"rollback_cause", "restore previous settings",
			)
		}
		return domain.WrapError(
			domain.CodeBackupRestoreFailed,
			"backup restore failed",
			errors.Join(err, rollbackErr, reloadErr),
		)
	}
	return nil
}
