package service

import (
	"context"
	"encoding/base64"
	"mime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSetShareContentDispositionReplacesUpstreamHeaderCaseInsensitively(t *testing.T) {
	for _, upstreamName := range []string{
		"Content-Disposition",
		"content-disposition",
		"cOnTeNt-DiSpOsItIoN",
	} {
		t.Run(upstreamName, func(t *testing.T) {
			result := domain.ShareRenderResult{Headers: map[string]string{
				upstreamName: "attachment; filename=unsafe.txt",
				"X-Test":     "preserved",
			}}

			setShareContentDisposition(&result, "safe.txt")

			require.Len(t, result.Headers, 2)
			require.Equal(t, "preserved", result.Headers["X-Test"])
			mediaType, params, err := mime.ParseMediaType(result.Headers["Content-Disposition"])
			require.NoError(t, err)
			require.Equal(t, "inline", mediaType)
			require.Equal(t, "safe.txt", params["filename"])
		})
	}
}

func TestAgeShareFilenameCollapsesTerminalSuffixes(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filename string
		expected string
	}{
		{name: "repeated lowercase", filename: "backup.age.age", expected: "backup.age"},
		{name: "mixed case", filename: "backup.AGE.age", expected: "backup.age"},
		{name: "suffixes only", filename: ".AGE.age", expected: "share.age"},
		{name: "empty", filename: "", expected: "share.age"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ageShareFilename(tt.filename))
		})
	}
}

func TestRenderShareDefaultsPersistedMissingSubscriptionFormatToBase64(t *testing.T) {
	ctx := context.Background()
	svc := New(WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "nodes", Type: domain.SubscriptionTypeLocal,
		Format: "uri-list", Content: "ss://aes-128-gcm:secret@example.com:8388#node",
	}))
	require.NoError(t, svc.metaStore.CreateShare(ctx, domain.Share{
		ID: "legacy-share", Name: "mobile",
		TargetKind: "subscription", TargetName: "nodes",
	}))

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "legacy-share"})
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(string(rendered.Body))
	require.NoError(t, err)
	require.Contains(t, string(decoded), "ss://")
}
