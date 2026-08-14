package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/afero"

	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const (
	EnvStorageBackend    = "SANDRONE_STORAGE_BACKEND"
	EnvS3Endpoint        = "SANDRONE_S3_ENDPOINT"
	EnvS3Region          = "SANDRONE_S3_REGION"
	EnvS3Bucket          = "SANDRONE_S3_BUCKET"
	EnvS3Prefix          = "SANDRONE_S3_PREFIX"
	EnvS3ForcePathStyle  = "SANDRONE_S3_FORCE_PATH_STYLE"
	EnvS3AccessKeyID     = "SANDRONE_S3_ACCESS_KEY_ID"
	EnvS3SecretAccessKey = "SANDRONE_S3_SECRET_ACCESS_KEY" // #nosec G101 -- environment variable name, not a credential
	EnvS3SessionToken    = "SANDRONE_S3_SESSION_TOKEN"     // #nosec G101 -- environment variable name, not a credential

	DefaultS3Prefix = "sandrone/"
)

type StorageBackend string

const (
	StorageFilesystem StorageBackend = "filesystem"
	StorageS3         StorageBackend = "s3"
)

type StorageConfig struct {
	Backend StorageBackend
	S3      store.S3Config
}

func StorageConfigFromEnv(env map[string]string) (StorageConfig, error) {
	backend := StorageBackend(strings.ToLower(strings.TrimSpace(env[EnvStorageBackend])))
	if backend == "" {
		backend = StorageFilesystem
	}
	if backend != StorageFilesystem && backend != StorageS3 {
		return StorageConfig{}, fmt.Errorf("%s must be filesystem or s3", EnvStorageBackend)
	}
	config := StorageConfig{Backend: backend}
	if backend == StorageFilesystem {
		return config, nil
	}
	forcePathStyle := false
	if raw := strings.TrimSpace(env[EnvS3ForcePathStyle]); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return StorageConfig{}, fmt.Errorf("%s must be a boolean", EnvS3ForcePathStyle)
		}
		forcePathStyle = value
	}
	prefix := env[EnvS3Prefix]
	if strings.TrimSpace(prefix) == "" {
		prefix = DefaultS3Prefix
	}
	normalized, err := store.NormalizeS3Config(store.S3Config{
		Endpoint:        env[EnvS3Endpoint],
		Region:          env[EnvS3Region],
		Bucket:          env[EnvS3Bucket],
		Prefix:          prefix,
		ForcePathStyle:  forcePathStyle,
		AccessKeyID:     env[EnvS3AccessKeyID],
		SecretAccessKey: env[EnvS3SecretAccessKey],
		SessionToken:    env[EnvS3SessionToken],
	})
	if err != nil {
		return StorageConfig{}, err
	}
	config.S3 = normalized
	return config, nil
}

func NewStore(ctx context.Context, dataDir string, cfg StorageConfig) (store.Store, error) {
	if cfg.Backend == "" {
		cfg.Backend = StorageFilesystem
	}
	switch cfg.Backend {
	case StorageFilesystem:
		if dataDir == "" {
			dataDir = DefaultDataDir
		}
		fs := afero.NewBasePathFs(afero.NewOsFs(), dataDir)
		return store.NewFSStore(fs), nil
	case StorageS3:
		return store.NewS3Store(ctx, cfg.S3)
	default:
		return nil, fmt.Errorf("unsupported storage backend %q", cfg.Backend)
	}
}
