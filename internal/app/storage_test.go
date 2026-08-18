package app

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/envconfig"
)

func testS3Env() map[string]string {
	return map[string]string{
		envconfig.StorageBackend:    "s3",
		envconfig.S3Endpoint:        "https://account.example.invalid",
		envconfig.S3Region:          "auto",
		envconfig.S3Bucket:          "bucket",
		envconfig.S3AccessKeyID:     "access-marker",
		envconfig.S3SecretAccessKey: "secret-marker",
		envconfig.S3SessionToken:    "session-marker",
	}
}

func TestStorageConfigFromEnvDefaultsToFilesystem(t *testing.T) {
	cfg, err := StorageConfigFromEnv(nil)
	require.NoError(t, err)
	require.Equal(t, StorageFilesystem, cfg.Backend)
}

func TestStorageConfigFromEnvBuildsS3Config(t *testing.T) {
	env := testS3Env()
	env[envconfig.S3ForcePathStyle] = "true"
	cfg, err := StorageConfigFromEnv(env)
	require.NoError(t, err)
	require.Equal(t, StorageS3, cfg.Backend)
	require.Equal(t, DefaultS3Prefix, cfg.S3.Prefix)
	require.True(t, cfg.S3.ForcePathStyle)
}

func TestStorageConfigFromEnvRejectsUnsupportedBackend(t *testing.T) {
	_, err := StorageConfigFromEnv(map[string]string{envconfig.StorageBackend: "r2"})
	require.ErrorContains(t, err, envconfig.StorageBackend)
}

func TestStorageConfigFromEnvRejectsInvalidBoolean(t *testing.T) {
	env := testS3Env()
	env[envconfig.S3ForcePathStyle] = "sometimes"
	_, err := StorageConfigFromEnv(env)
	require.ErrorContains(t, err, envconfig.S3ForcePathStyle)
}

func TestStorageConfigFromEnvDoesNotLeakSecrets(t *testing.T) {
	env := testS3Env()
	env[envconfig.S3Endpoint] = "https://user:password@example.invalid"
	_, err := StorageConfigFromEnv(env)
	require.Error(t, err)
	for _, secret := range []string{env[envconfig.S3AccessKeyID], env[envconfig.S3SecretAccessKey], env[envconfig.S3SessionToken], "password"} {
		require.False(t, strings.Contains(err.Error(), secret))
	}
}

func TestNewStoreUsesFilesystemDataDir(t *testing.T) {
	storage, err := NewStore(context.Background(), t.TempDir(), StorageConfig{})
	require.NoError(t, err)
	require.NoError(t, storage.Write(context.Background(), "settings.json", []byte("{}")))
}
