package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSubscriptionTrafficIgnoresProfileHeadersWithoutUsage(t *testing.T) {
	svc := New(WithClock(func() time.Time { return time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC) }))
	items, warnings := svc.subscriptionTrafficFromRemote(domain.Subscription{
		Name:   "provider",
		Remote: &domain.RemoteInput{URL: "https://example.test/sub"},
	}, &remoteInputResult{
		SourceRef: domain.SourceRef{URL: "https://example.test/sub"},
		Headers: http.Header{
			"Profile-Web-Page-Url": []string{"https://panel.example.test"},
			"Plan-Name":            []string{"Example plan"},
		},
	})

	require.Empty(t, warnings)
	require.Empty(t, items)
}

func TestSubscriptionTrafficIgnoresMetadataOnlyUserinfo(t *testing.T) {
	svc := New(WithClock(func() time.Time { return time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC) }))
	items, warnings := svc.subscriptionTrafficFromRemote(domain.Subscription{
		Name:   "provider",
		Remote: &domain.RemoteInput{URL: "https://example.test/sub"},
	}, &remoteInputResult{
		SourceRef: domain.SourceRef{URL: "https://example.test/sub"},
		Headers: http.Header{
			"Subscription-Userinfo": []string{"app_url=https://panel.example.test; plan_name=Example plan"},
		},
	})

	require.Empty(t, warnings)
	require.Empty(t, items)
}

func TestSubscriptionTrafficParsesDecimalAndScientificNotation(t *testing.T) {
	svc := New(WithClock(func() time.Time { return time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC) }))
	items, warnings := svc.subscriptionTrafficFromRemote(domain.Subscription{
		Name:   "provider",
		Remote: &domain.RemoteInput{URL: "https://example.test/sub"},
	}, &remoteInputResult{
		SourceRef: domain.SourceRef{URL: "https://example.test/sub"},
		Headers: http.Header{
			"Subscription-Userinfo": []string{"upload=1.5e3; download=2.5; total=10000.9; expire=1780272000"},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, items, 1)
	require.Equal(t, int64(1500), items[0].UploadBytes)
	require.Equal(t, int64(2), items[0].DownloadBytes)
	require.Equal(t, int64(1502), items[0].UsedBytes)
	require.NotNil(t, items[0].TotalBytes)
	require.Equal(t, int64(10000), *items[0].TotalBytes)
}

func TestSubscriptionTrafficRejectsNegativeAndOverflowUsage(t *testing.T) {
	svc := New(WithClock(func() time.Time { return time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC) }))
	items, warnings := svc.subscriptionTrafficFromRemote(domain.Subscription{
		Name:   "provider",
		Remote: &domain.RemoteInput{URL: "https://example.test/sub"},
	}, &remoteInputResult{
		SourceRef: domain.SourceRef{URL: "https://example.test/sub"},
		Headers: http.Header{
			"Subscription-Userinfo": []string{"upload=9223372036854775807; download=1; total=-1"},
		},
	})

	require.Len(t, warnings, 2)
	require.Empty(t, items)
}
