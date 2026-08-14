package app_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestS3IntegrationRuntimePersistenceAndBackup(t *testing.T) {
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
	prefix += "/app-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "/"
	storageConfig := app.StorageConfig{
		Backend: app.StorageS3,
		S3: store.S3Config{
			Endpoint:        os.Getenv("SANDRONE_TEST_S3_ENDPOINT"),
			Region:          os.Getenv("SANDRONE_TEST_S3_REGION"),
			Bucket:          os.Getenv("SANDRONE_TEST_S3_BUCKET"),
			Prefix:          prefix,
			ForcePathStyle:  forcePathStyle,
			AccessKeyID:     os.Getenv("SANDRONE_TEST_S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("SANDRONE_TEST_S3_SECRET_ACCESS_KEY"),
			SessionToken:    os.Getenv("SANDRONE_TEST_S3_SESSION_TOKEN"),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rawStore, err := app.NewStore(ctx, "", storageConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		entries, listErr := rawStore.List(context.Background(), "")
		if listErr != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir {
				_ = rawStore.Delete(context.Background(), entry.Key)
			}
		}
	})

	newRuntime := func() *app.Runtime {
		runtime, runtimeErr := app.NewRuntimeContext(ctx, app.Config{
			Storage: storageConfig,
			HTTP:    app.HTTPConfig{Listen: app.DefaultListen},
		}, nil)
		require.NoError(t, runtimeErr)
		return runtime
	}
	first := newRuntime()
	require.NoError(t, first.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "integration",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://example.invalid\n",
	}))
	require.NoError(t, first.Service.PutFile(ctx, domain.FileSpec{
		Name:   "integration.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "integration\n"},
	}))
	share, err := first.Service.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "integration-share",
		TargetKind: "file",
		TargetName: "integration.txt",
	})
	require.NoError(t, err)
	backup, err := first.Service.ExportBackup(ctx)
	require.NoError(t, err)

	second := newRuntime()
	_, err = second.Service.GetSubscription(ctx, "integration")
	require.NoError(t, err)
	_, err = second.Service.GetShare(ctx, share.ID)
	require.NoError(t, err)
	require.NoError(t, second.Service.DeleteSubscription(ctx, "integration"))
	require.NoError(t, second.Service.RestoreBackup(ctx, backup.Body))
	_, err = second.Service.GetSubscription(ctx, "integration")
	require.NoError(t, err)
}
