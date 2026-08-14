package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func integrationS3Config(t *testing.T) S3Config {
	t.Helper()
	if os.Getenv("SANDRONE_TEST_S3") != "1" {
		t.Skip("set SANDRONE_TEST_S3=1 to run S3 integration tests")
	}
	forcePathStyle := false
	if raw := os.Getenv("SANDRONE_TEST_S3_FORCE_PATH_STYLE"); raw != "" {
		value, err := strconv.ParseBool(raw)
		require.NoError(t, err)
		forcePathStyle = value
	}
	prefix := os.Getenv("SANDRONE_TEST_S3_PREFIX")
	require.NotEmpty(t, prefix)
	prefix += "/integration-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "/"
	return S3Config{
		Endpoint:        os.Getenv("SANDRONE_TEST_S3_ENDPOINT"),
		Region:          os.Getenv("SANDRONE_TEST_S3_REGION"),
		Bucket:          os.Getenv("SANDRONE_TEST_S3_BUCKET"),
		Prefix:          prefix,
		ForcePathStyle:  forcePathStyle,
		AccessKeyID:     os.Getenv("SANDRONE_TEST_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("SANDRONE_TEST_S3_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("SANDRONE_TEST_S3_SESSION_TOKEN"),
	}
}

func TestS3IntegrationStoreContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	storage, err := NewS3Store(ctx, integrationS3Config(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		entries, listErr := storage.List(context.Background(), "")
		if listErr != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir {
				_ = storage.Delete(context.Background(), entry.Key)
			}
		}
	})

	require.NoError(t, storage.Write(ctx, "settings.json", []byte("first")))
	require.NoError(t, storage.WriteAtomic(ctx, "settings.json", []byte("second"), 0o600))
	body, err := storage.Read(ctx, "settings.json")
	require.NoError(t, err)
	require.Equal(t, []byte("second"), body)
	require.NoError(t, storage.Write(ctx, "files/nested/demo.json", []byte("demo")))
	entries, err := storage.List(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	dir, err := storage.Stat(ctx, "files/nested")
	require.NoError(t, err)
	require.True(t, dir.IsDir)
	require.NoError(t, storage.Delete(ctx, "settings.json"))
	_, err = storage.Read(ctx, "settings.json")
	require.ErrorIs(t, err, os.ErrNotExist)
}
