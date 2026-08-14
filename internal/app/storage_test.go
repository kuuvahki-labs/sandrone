package app

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testS3Env() map[string]string {
	return map[string]string{
		EnvStorageBackend:    "s3",
		EnvS3Endpoint:        "https://account.example.invalid",
		EnvS3Region:          "auto",
		EnvS3Bucket:          "bucket",
		EnvS3AccessKeyID:     "access-marker",
		EnvS3SecretAccessKey: "secret-marker",
		EnvS3SessionToken:    "session-marker",
	}
}

func TestStorageConfigFromEnvDefaultsToFilesystem(t *testing.T) {
	cfg, err := StorageConfigFromEnv(nil)
	require.NoError(t, err)
	require.Equal(t, StorageFilesystem, cfg.Backend)
}

func TestStorageConfigFromEnvBuildsS3Config(t *testing.T) {
	env := testS3Env()
	env[EnvS3ForcePathStyle] = "true"
	cfg, err := StorageConfigFromEnv(env)
	require.NoError(t, err)
	require.Equal(t, StorageS3, cfg.Backend)
	require.Equal(t, DefaultS3Prefix, cfg.S3.Prefix)
	require.True(t, cfg.S3.ForcePathStyle)
}

func TestStorageConfigFromEnvRejectsUnsupportedBackend(t *testing.T) {
	_, err := StorageConfigFromEnv(map[string]string{EnvStorageBackend: "r2"})
	require.ErrorContains(t, err, EnvStorageBackend)
}

func TestStorageConfigFromEnvRejectsInvalidBoolean(t *testing.T) {
	env := testS3Env()
	env[EnvS3ForcePathStyle] = "sometimes"
	_, err := StorageConfigFromEnv(env)
	require.ErrorContains(t, err, EnvS3ForcePathStyle)
}

func TestStorageConfigFromEnvDoesNotLeakSecrets(t *testing.T) {
	env := testS3Env()
	env[EnvS3Endpoint] = "https://user:password@example.invalid"
	_, err := StorageConfigFromEnv(env)
	require.Error(t, err)
	for _, secret := range []string{env[EnvS3AccessKeyID], env[EnvS3SecretAccessKey], env[EnvS3SessionToken], "password"} {
		require.False(t, strings.Contains(err.Error(), secret))
	}
}

func TestNewStoreUsesFilesystemDataDir(t *testing.T) {
	storage, err := NewStore(context.Background(), t.TempDir(), StorageConfig{})
	require.NoError(t, err)
	require.NoError(t, storage.Write(context.Background(), "settings.json", []byte("{}")))
}
