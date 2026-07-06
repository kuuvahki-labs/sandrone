package service_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestServiceRuntimeSettingsDefaultsAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	defaults, err := svc.GetRuntimeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "sandrone/0.1.0", defaults.RemoteDefaults.UserAgent)
	require.Equal(t, 15000, defaults.RemoteDefaults.TimeoutMS)
	require.Equal(t, "protocol", defaults.ProbeDefaults.Layer)
	require.Equal(t, "auto", defaults.ProbeDefaults.Method)
	require.Equal(t, "sing-box", defaults.ProbeDefaults.Core)
	require.Equal(t, "http://www.gstatic.com/generate_204", defaults.ProbeDefaults.URL)
	require.Equal(t, "time.apple.com", defaults.ProbeDefaults.NTPServer)
	require.Equal(t, 5000, defaults.ProbeDefaults.TimeoutMS)
	require.Equal(t, 1, defaults.ProbeDefaults.Attempts)
	require.Equal(t, 10, defaults.ProbeDefaults.Concurrency)
	require.Equal(t, 0, defaults.CacheDefaults.RemoteFetchTTLSeconds)
	require.Equal(t, 60, defaults.CacheDefaults.SubscriptionTrafficTTLSeconds)

	settings := domain.RuntimeSettings{
		RemoteDefaults: domain.RemoteDefaults{
			UserAgent: "Sandrone Test",
			Proxy:     "http://127.0.0.1:7890",
			TimeoutMS: 8000,
		},
		ProbeDefaults: domain.ProbeDefaults{
			Layer:           "proxy",
			Method:          "url_test",
			Core:            "sing-box",
			URL:             "https://example.com/generate_204",
			NTPServer:       "time.cloudflare.com",
			TimeoutMS:       9000,
			Attempts:        2,
			Concurrency:     12,
			CacheTTLSeconds: 300,
		},
		CacheDefaults: domain.CacheDefaults{
			RemoteFetchTTLSeconds:         120,
			SubscriptionTrafficTTLSeconds: 15,
		},
	}
	require.NoError(t, svc.PutRuntimeSettings(ctx, settings))

	got, err := svc.GetRuntimeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, settings, got)
}

func TestServiceRuntimeSettingsMigratesExactLegacyDefaultUserAgent(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	require.NoError(t, store.NewMetaStore(resourceStore).PutRuntimeSettings(ctx, domain.RuntimeSettings{
		RemoteDefaults: domain.RemoteDefaults{UserAgent: "sandrone/0"},
	}))
	svc := service.New(service.WithStore(resourceStore))

	settings, err := svc.GetRuntimeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "sandrone/0.1.0", settings.RemoteDefaults.UserAgent)
}

func TestServiceRuntimeSettingsPreservesCustomUserAgent(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	require.NoError(t, store.NewMetaStore(resourceStore).PutRuntimeSettings(ctx, domain.RuntimeSettings{
		RemoteDefaults: domain.RemoteDefaults{UserAgent: "Sandrone Custom"},
	}))
	svc := service.New(service.WithStore(resourceStore))

	settings, err := svc.GetRuntimeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "Sandrone Custom", settings.RemoteDefaults.UserAgent)
}

func TestServiceRuntimeSettingsRejectsNegativeCacheDefaults(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	for _, tc := range []struct {
		name     string
		settings domain.RuntimeSettings
	}{
		{
			name: "remote fetch ttl",
			settings: domain.RuntimeSettings{
				CacheDefaults: domain.CacheDefaults{RemoteFetchTTLSeconds: -1},
			},
		},
		{
			name: "subscription traffic ttl",
			settings: domain.RuntimeSettings{
				CacheDefaults: domain.CacheDefaults{SubscriptionTrafficTTLSeconds: -1},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.PutRuntimeSettings(ctx, tc.settings)
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
		})
	}
}
