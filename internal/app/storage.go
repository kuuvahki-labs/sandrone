package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/afero"

	"github.com/kuuvahki-labs/sandrone/internal/envconfig"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const DefaultS3Prefix = "sandrone/"

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
	backend := StorageBackend(strings.ToLower(strings.TrimSpace(env[envconfig.StorageBackend])))
	if backend == "" {
		backend = StorageFilesystem
	}
	if backend != StorageFilesystem && backend != StorageS3 {
		return StorageConfig{}, fmt.Errorf("%s must be filesystem or s3", envconfig.StorageBackend)
	}
	config := StorageConfig{Backend: backend}
	if backend == StorageFilesystem {
		return config, nil
	}
	forcePathStyle := false
	if raw := strings.TrimSpace(env[envconfig.S3ForcePathStyle]); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return StorageConfig{}, fmt.Errorf("%s must be a boolean", envconfig.S3ForcePathStyle)
		}
		forcePathStyle = value
	}
	prefix := env[envconfig.S3Prefix]
	if strings.TrimSpace(prefix) == "" {
		prefix = DefaultS3Prefix
	}
	normalized, err := store.NormalizeS3Config(store.S3Config{
		Endpoint:        env[envconfig.S3Endpoint],
		Region:          env[envconfig.S3Region],
		Bucket:          env[envconfig.S3Bucket],
		Prefix:          prefix,
		ForcePathStyle:  forcePathStyle,
		AccessKeyID:     env[envconfig.S3AccessKeyID],
		SecretAccessKey: env[envconfig.S3SecretAccessKey],
		SessionToken:    env[envconfig.S3SessionToken],
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
