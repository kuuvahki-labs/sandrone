package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const (
	backupFormat               = "sandrone-store-backup"
	backupStorageSchemaVersion = 1
	backupManifestName         = "manifest.json"
	backupDataPrefix           = "data/"
	backupMaxCompressedBytes   = 32 << 20
	backupMaxDecodedBytes      = 64 << 20
	backupMaxEntries           = 10_000
)

type backupManifest struct {
	Format               string `json:"format"`
	StorageSchemaVersion int    `json:"storage_schema_version"`
	CreatedAt            string `json:"created_at"`
	AppVersion           string `json:"app_version"`
}

type backupEntry struct {
	key  string
	body []byte
}

type backupSnapshot struct {
	files        map[string][]byte
	currentPaths []string
}

type backupStoreOperationError struct {
	operation string
	cause     error
}

func (e *backupStoreOperationError) Error() string {
	return e.operation + ": " + e.cause.Error()
}

func (e *backupStoreOperationError) Unwrap() error { return e.cause }

// ExportBackup snapshots every non-cache Store file and encodes its raw key
// and bytes in the versioned backup ZIP contract.
func (s *Service) ExportBackup(ctx context.Context) (*domain.BackupExportResult, error) {
	if s.storeCoordinator == nil {
		return nil, errors.New("backup Store is not configured")
	}

	createdAt := s.now().UTC()
	var entries []backupEntry
	err := s.storeCoordinator.View(ctx, func(resourceStore store.Store) error {
		var err error
		entries, err = readBackupEntries(ctx, resourceStore)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("export backup: %w", err)
	}

	body, err := encodeBackupArchive(entries, createdAt)
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
	files, err := decodeBackupArchive(body)
	if err != nil {
		return err
	}
	if s.storeCoordinator == nil {
		return domain.NewError(domain.CodeBackupRestoreFailed, "backup Store is not configured")
	}

	return s.storeCoordinator.Update(ctx, func(resourceStore store.Store) error {
		snapshot, err := snapshotBackupStore(ctx, resourceStore)
		if err != nil {
			return domain.WrapError(domain.CodeBackupRestoreFailed, "backup restore failed", err)
		}

		mutationCtx := context.WithoutCancel(ctx)
		if err := replaceBackupStoreFiles(mutationCtx, resourceStore, snapshot.currentPaths, files); err != nil {
			rollbackErr := rollbackBackupStore(mutationCtx, resourceStore, snapshot.files)
			if rollbackErr != nil {
				s.log(mutationCtx, slog.LevelError, "service backup restore rollback failed",
					"restore_cause", backupStoreOperation(err),
					"rollback_cause", backupStoreOperation(rollbackErr),
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
}

func readBackupEntries(ctx context.Context, resourceStore store.Store) ([]backupEntry, error) {
	listed, err := resourceStore.List(ctx, "")
	if err != nil {
		return nil, err
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Key < listed[j].Key })
	entries := make([]backupEntry, 0, len(listed))
	seen := make(map[string]struct{}, len(listed))
	for _, item := range listed {
		if item.IsDir {
			continue
		}
		key, err := store.CleanKey(item.Key)
		if err != nil || key != item.Key {
			return nil, errors.New("Store returned an unsafe backup key")
		}
		if _, ok := seen[key]; ok {
			return nil, errors.New("Store returned a duplicate backup key")
		}
		seen[key] = struct{}{}
		if isBackupCacheKey(key) {
			continue
		}
		body, err := resourceStore.Read(ctx, key)
		if err != nil {
			return nil, err
		}
		entries = append(entries, backupEntry{key: key, body: body})
	}
	return entries, nil
}

func encodeBackupArchive(entries []backupEntry, createdAt time.Time) ([]byte, error) {
	manifestBody, err := json.Marshal(backupManifest{
		Format:               backupFormat,
		StorageSchemaVersion: backupStorageSchemaVersion,
		CreatedAt:            createdAt.Format(time.RFC3339),
		AppVersion:           buildinfo.Version(),
	})
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	if err := writeBackupZipFile(writer, backupManifestName, manifestBody, createdAt); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if err := writeBackupZipFile(writer, backupDataPrefix+entry.key, entry.body, createdAt); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func writeBackupZipFile(writer *zip.Writer, name string, body []byte, modified time.Time) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(modified.UTC())
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(body)
	return err
}

func decodeBackupArchive(body []byte) (map[string][]byte, error) {
	if len(body) == 0 {
		return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive is required")
	}
	if len(body) > backupMaxCompressedBytes {
		return nil, domain.NewError(domain.CodeBackupTooLarge, "compressed backup archive is too large")
	}

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, domain.WrapError(domain.CodeBackupInvalid, "backup archive is invalid", err)
	}

	files := make(map[string][]byte)
	var manifestBody []byte
	manifestSeen := false
	dataEntries := 0
	decodedBytes := int64(0)
	for _, member := range reader.File {
		if member.FileInfo().IsDir() || !member.Mode().IsRegular() {
			return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains a non-file member")
		}

		isManifest := member.Name == backupManifestName
		if isManifest {
			if manifestSeen {
				return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains a duplicate manifest")
			}
			manifestSeen = true
		} else if !strings.HasPrefix(member.Name, backupDataPrefix) {
			return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains an unknown member")
		}

		if !isManifest {
			dataEntries++
			if dataEntries > backupMaxEntries {
				return nil, domain.NewError(domain.CodeBackupTooLarge, "backup archive contains too many data entries")
			}
		}

		memberBody, err := readBackupZipMember(member, int64(backupMaxDecodedBytes)-decodedBytes)
		if err != nil {
			return nil, err
		}
		decodedBytes += int64(len(memberBody))
		if isManifest {
			manifestBody = memberBody
			continue
		}

		key := strings.TrimPrefix(member.Name, backupDataPrefix)
		cleaned, err := store.CleanKey(key)
		if err != nil || cleaned != key {
			return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains an unsafe data key")
		}
		if isBackupCacheKey(key) {
			return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains a cache entry")
		}
		if _, exists := files[key]; exists {
			return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive contains a duplicate data key")
		}
		files[key] = memberBody
	}

	if !manifestSeen {
		return nil, domain.NewError(domain.CodeBackupInvalid, "backup archive has no manifest")
	}
	if err := validateBackupFileTree(files); err != nil {
		return nil, err
	}
	if err := validateBackupManifest(manifestBody); err != nil {
		return nil, err
	}
	return files, nil
}

func validateBackupFileTree(files map[string][]byte) error {
	for key := range files {
		for separator := strings.IndexByte(key, '/'); separator >= 0; {
			if _, conflicts := files[key[:separator]]; conflicts {
				return domain.NewError(domain.CodeBackupInvalid, "backup archive contains conflicting data keys")
			}
			next := strings.IndexByte(key[separator+1:], '/')
			if next < 0 {
				break
			}
			separator += next + 1
		}
	}
	return nil
}

func readBackupZipMember(member *zip.File, remaining int64) ([]byte, error) {
	if remaining < 0 {
		return nil, domain.NewError(domain.CodeBackupTooLarge, "decoded backup archive is too large")
	}
	opened, err := member.Open()
	if err != nil {
		return nil, domain.WrapError(domain.CodeBackupInvalid, "backup archive member is unreadable", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(opened, remaining+1))
	closeErr := opened.Close()
	if int64(len(body)) > remaining {
		return nil, domain.NewError(domain.CodeBackupTooLarge, "decoded backup archive is too large")
	}
	if readErr != nil {
		return nil, domain.WrapError(domain.CodeBackupInvalid, "backup archive member is unreadable", readErr)
	}
	if closeErr != nil {
		return nil, domain.WrapError(domain.CodeBackupInvalid, "backup archive member is unreadable", closeErr)
	}
	return body, nil
}

func validateBackupManifest(body []byte) error {
	fields, err := decodeBackupManifestFields(body)
	if err != nil {
		return domain.WrapError(domain.CodeBackupInvalid, "backup manifest is invalid", err)
	}
	expected := []string{"format", "storage_schema_version", "created_at", "app_version"}
	if len(fields) != len(expected) {
		return domain.NewError(domain.CodeBackupInvalid, "backup manifest fields are invalid")
	}
	for _, field := range expected {
		if _, ok := fields[field]; !ok {
			return domain.NewError(domain.CodeBackupInvalid, "backup manifest fields are invalid")
		}
	}

	var manifest struct {
		Format               *string `json:"format"`
		StorageSchemaVersion *int    `json:"storage_schema_version"`
		CreatedAt            *string `json:"created_at"`
		AppVersion           *string `json:"app_version"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return domain.WrapError(domain.CodeBackupInvalid, "backup manifest is invalid", err)
	}
	if manifest.Format == nil || manifest.StorageSchemaVersion == nil || manifest.CreatedAt == nil || manifest.AppVersion == nil {
		return domain.NewError(domain.CodeBackupInvalid, "backup manifest fields are invalid")
	}
	if *manifest.Format != backupFormat {
		return domain.NewError(domain.CodeBackupInvalid, "backup format is invalid")
	}
	if *manifest.StorageSchemaVersion != backupStorageSchemaVersion {
		return domain.NewError(domain.CodeBackupIncompatible, "backup storage schema is incompatible")
	}
	if _, err := time.Parse(time.RFC3339, *manifest.CreatedAt); err != nil {
		return domain.WrapError(domain.CodeBackupInvalid, "backup creation timestamp is invalid", err)
	}
	if *manifest.AppVersion == "" {
		return domain.NewError(domain.CodeBackupInvalid, "backup app version is invalid")
	}
	return nil
}

func decodeBackupManifestFields(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("manifest must be a JSON object")
	}

	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, errors.New("manifest field name is invalid")
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, errors.New("manifest contains a duplicate field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields[name] = value
	}
	if token, err = decoder.Token(); err != nil {
		return nil, err
	} else if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' {
		return nil, errors.New("manifest object is not closed")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("manifest contains trailing JSON")
	}
	return fields, nil
}

func snapshotBackupStore(ctx context.Context, resourceStore store.Store) (backupSnapshot, error) {
	listed, err := resourceStore.List(ctx, "")
	if err != nil {
		return backupSnapshot{}, &backupStoreOperationError{operation: "list current Store files", cause: err}
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Key < listed[j].Key })
	snapshot := backupSnapshot{files: make(map[string][]byte)}
	for _, item := range listed {
		snapshot.currentPaths = append(snapshot.currentPaths, item.Key)
		if item.IsDir {
			continue
		}
		if isBackupCacheKey(item.Key) {
			continue
		}
		body, err := resourceStore.Read(ctx, item.Key)
		if err != nil {
			return backupSnapshot{}, &backupStoreOperationError{operation: "read current Store files", cause: err}
		}
		snapshot.files[item.Key] = body
	}
	return snapshot, nil
}

func replaceBackupStoreFiles(ctx context.Context, resourceStore store.Store, currentPaths []string, replacement map[string][]byte) error {
	deletionKeys := append([]string(nil), currentPaths...)
	sort.Slice(deletionKeys, func(i, j int) bool {
		leftDepth := strings.Count(deletionKeys[i], "/")
		rightDepth := strings.Count(deletionKeys[j], "/")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return deletionKeys[i] > deletionKeys[j]
	})
	for _, key := range deletionKeys {
		if err := resourceStore.Delete(ctx, key); err != nil {
			return &backupStoreOperationError{operation: "delete current Store files", cause: err}
		}
	}
	keys := make([]string, 0, len(replacement))
	for key := range replacement {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := resourceStore.Write(ctx, key, replacement[key]); err != nil {
			return &backupStoreOperationError{operation: "write replacement Store files", cause: err}
		}
	}
	return nil
}

func rollbackBackupStore(ctx context.Context, resourceStore store.Store, snapshot map[string][]byte) error {
	listed, err := resourceStore.List(ctx, "")
	if err != nil {
		return &backupStoreOperationError{operation: "list Store files for rollback", cause: err}
	}
	keys := make([]string, 0, len(listed))
	for _, item := range listed {
		keys = append(keys, item.Key)
	}
	if err := replaceBackupStoreFiles(ctx, resourceStore, keys, snapshot); err != nil {
		return &backupStoreOperationError{operation: "restore previous Store files", cause: err}
	}
	return nil
}

func backupStoreOperation(err error) string {
	var operationErr *backupStoreOperationError
	if errors.As(err, &operationErr) {
		return operationErr.operation
	}
	return "unknown Store operation"
}

func isBackupCacheKey(key string) bool {
	return key == "cache" || strings.HasPrefix(key, "cache/")
}
