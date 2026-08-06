package service_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/metacubex/age"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceShareFileLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 13, 8, 9, 10, 0, time.UTC)
	svc := service.New(service.WithFS(afero.NewMemMapFs()), service.WithClock(func() time.Time {
		return now
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "exports/default.yaml",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "proxies: []\n"},
	}))

	created, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		Name:        "default mihomo",
		TargetKind:  "file",
		TargetName:  "exports/default.yaml",
		ContentType: "application/yaml",
		Meta:        map[string]string{"target": "mihomo"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, now, created.CreatedAt)
	require.Equal(t, "file", created.TargetKind)

	list, err := svc.ListShares(ctx)
	require.NoError(t, err)
	require.Len(t, list.Shares, 1)
	require.Equal(t, created.ID, list.Shares[0].ID)

	now = now.Add(time.Minute)
	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, "application/yaml", rendered.ContentType)
	require.Equal(t, "proxies: []\n", string(rendered.Body))
	requireInlineShareFilename(t, rendered.Headers, "default mihomo")
	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: created.ID})
	require.NoError(t, err)
	stored, err := svc.GetShare(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.UpdatedAt, stored.UpdatedAt)

	require.NoError(t, svc.DeleteShare(ctx, created.ID))
	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: created.ID})
	require.True(t, os.IsNotExist(err))
}

func TestServiceListSharesUsesStoredFileShareNameForPresentation(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "generated-client",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "ok"},
	}))
	share, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "file-share",
		Name:       "shadowrocket.conf",
		TargetKind: "file",
		TargetName: "generated-client",
	})
	require.NoError(t, err)

	result, err := svc.ListShares(ctx)
	require.NoError(t, err)
	require.Equal(t, domain.SharePresentation{
		PublicFilename: "shadowrocket.conf",
	}, result.Presentations[share.ID])
}

func TestServiceListSharesIncludesEveryRegisteredSubscriptionFormatPresentation(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	share, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:           "nodes-share",
		Name:         "mobile.conf",
		TargetKind:   "subscription",
		TargetName:   "nodes",
		TargetFormat: "uri-list",
	})
	require.NoError(t, err)

	result, err := svc.ListShares(ctx)
	require.NoError(t, err)
	require.Equal(t, domain.SharePresentation{
		PublicFilename: "mobile.txt",
		FormatFilenames: map[string]string{
			"base64":               "mobile.txt",
			"uri-list":             "mobile.txt",
			"mihomo-proxies":       "mobile.yaml",
			"shadowrocket-proxies": "mobile.conf",
			"sing-box-outbounds":   "mobile.json",
			"json-nodes":           "mobile.json",
		},
	}, result.Presentations[share.ID])
}

func TestServiceCreateShareReturnsCanonicalPresentation(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "provider", Type: domain.SubscriptionTypeLocal,
		Format: "uri-list", Content: "ss://aes-128-gcm:secret@example.com:8388#node",
	}))

	created, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "mobile-share", Name: "mobile.conf", TargetKind: "subscription",
		TargetName: "provider", TargetFormat: "mihomo-proxies",
	})
	require.NoError(t, err)
	require.Equal(t, "mobile.yaml", created.Presentation.PublicFilename)
	require.Equal(t, "mobile.txt", created.Presentation.FormatFilenames["uri-list"])
	require.Equal(t, "mobile.conf", created.Presentation.FormatFilenames["shadowrocket-proxies"])
}

func TestServiceDefaultsNewSubscriptionSharesToBase64(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "nodes", Type: domain.SubscriptionTypeLocal,
		Format: "uri-list", Content: "ss://aes-128-gcm:secret@example.com:8388#node",
	}))

	created, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "default-share", Name: "mobile",
		TargetKind: "subscription", TargetName: "nodes",
	})
	require.NoError(t, err)
	require.Equal(t, "base64", created.Share.TargetFormat)
	require.Equal(t, "mobile.txt", created.Presentation.PublicFilename)
	require.Equal(t, "mobile.txt", created.Presentation.FormatFilenames["base64"])
	require.Equal(t, "mobile.txt", created.Presentation.FormatFilenames["uri-list"])

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: created.Share.ID})
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(string(rendered.Body))
	require.NoError(t, err)
	require.Contains(t, string(decoded), "ss://")
}

func TestServicePreservesExplicitURIListShareFormat(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "nodes", Type: domain.SubscriptionTypeLocal,
		Format: "uri-list", Content: "ss://aes-128-gcm:secret@example.com:8388#node",
	}))

	created, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "plain-share", Name: "mobile",
		TargetKind: "subscription", TargetName: "nodes", TargetFormat: "uri-list",
	})
	require.NoError(t, err)
	require.Equal(t, "uri-list", created.Share.TargetFormat)

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: created.Share.ID})
	require.NoError(t, err)
	require.Contains(t, string(rendered.Body), "ss://")
}

func TestServiceShareEncryptsForOneAgeX25519Recipient(t *testing.T) {
	ctx := context.Background()
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{Name: "backup.AGE.age", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "secret body"}}))

	share, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "encrypted", TargetKind: "file", TargetName: "backup.AGE.age",
		AgeRecipient: identity.Recipient().String(),
	})
	require.NoError(t, err)
	require.Equal(t, identity.Recipient().String(), share.AgeRecipient)
	require.Equal(t, "backup.age", share.Presentation.PublicFilename)

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: share.ID})
	require.NoError(t, err)
	require.Equal(t, "application/age", rendered.ContentType)
	require.Equal(t, "application/octet-stream", rendered.Headers["X-Sandrone-Original-Content-Type"])
	filename := requireInlineShareFilename(t, rendered.Headers, "backup.age")
	require.Equal(t, 1, strings.Count(strings.ToLower(filename), ".age"))
	reader, err := age.Decrypt(bytes.NewReader(rendered.Body), identity)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "secret body", string(plaintext))
}

func TestServiceShareRejectsFilenameMismatch(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "client",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "ok"},
	}))
	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "file-share",
		Name:       "shadowrocket.conf",
		TargetKind: "file",
		TargetName: "client",
	})
	require.NoError(t, err)

	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{
		ID:                "file-share",
		PresentedFilename: "wrong.conf",
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{
		ID:                "file-share",
		PresentedFilename: "shadowrocket.conf",
	})
	require.NoError(t, err)
	requireInlineShareFilename(t, rendered.Headers, "shadowrocket.conf")
}

func TestServiceRenderFileShareUsesProcessedFileContent(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "client",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:    "inline",
			Content: "proxies: []\n",
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource("function main(input) { input.file.name = 'mobile.prod.yaml'; input.file.content += '# processed\\n'; return input; }"),
			}),
		}},
	}))

	share, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:          "client-share",
		Name:        "client-share",
		TargetKind:  "file",
		TargetName:  "client",
		ContentType: "application/x-client-config",
	})
	require.NoError(t, err)
	require.Equal(t, "file", share.TargetKind)

	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "client-share", Format: "json-nodes"})
	require.NoError(t, err)
	require.Equal(t, "application/x-client-config", rendered.ContentType)
	require.Equal(t, "proxies: []\n# processed\n", string(rendered.Body))
	requireInlineShareFilename(t, rendered.Headers, "client-share")
}

func TestServiceRenderSubscriptionShareSupportsDefaultAndOverrideFormat(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:           "nodes-share",
		Name:         "mobile.yaml",
		TargetKind:   "subscription",
		TargetName:   "nodes",
		TargetFormat: "uri-list",
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name        string
		format      string
		contentType string
		filename    string
		bodyPart    string
	}{
		{name: "stored uri-list default", contentType: "text/plain", filename: "mobile.txt", bodyPart: "node-a"},
		{name: "mihomo proxies", format: "mihomo-proxies", contentType: "application/yaml", filename: "mobile.yaml", bodyPart: "proxies:"},
		{name: "shadowrocket proxies", format: "shadowrocket-proxies", contentType: "text/plain; charset=utf-8", filename: "mobile.conf", bodyPart: "[Proxy]"},
		{name: "sing-box outbounds", format: "sing-box-outbounds", contentType: "application/json", filename: "mobile.json", bodyPart: "outbounds"},
		{name: "json nodes", format: "json-nodes", contentType: "application/json", filename: "mobile.json", bodyPart: `"name"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "nodes-share", Format: tt.format})
			require.NoError(t, err)
			require.Equal(t, tt.contentType, out.ContentType)
			require.Contains(t, string(out.Body), tt.bodyPart)
			requireInlineShareFilename(t, out.Headers, tt.filename)
		})
	}

	for _, tt := range []struct {
		id       string
		name     string
		format   string
		expected string
	}{
		{id: "yml-share", name: "mobile.yml", format: "mihomo-proxies", expected: "mobile.yaml"},
		{id: "internal-dot-share", name: "mobile.prod", format: "mihomo-proxies", expected: "mobile.prod.yaml"},
		{id: "case-insensitive-share", name: "Mobile.JSON.AGE", format: "uri-list", expected: "Mobile.txt"},
		{id: "trimmed-terminal-share", name: "mobile.yaml.", format: "uri-list", expected: "mobile.txt"},
		{id: "suffix-only-yaml-share", name: ".yaml", format: "uri-list", expected: "nodes.txt"},
		{id: "suffix-only-yaml-case-share", name: ".YaMl", format: "uri-list", expected: "nodes.txt"},
		{id: "suffix-only-age-share", name: ".age", format: "uri-list", expected: "nodes.txt"},
		{id: "suffix-only-age-case-share", name: ".AgE", format: "uri-list", expected: "nodes.txt"},
	} {
		t.Run(tt.id, func(t *testing.T) {
			_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
				ID: tt.id, Name: tt.name, TargetKind: "subscription", TargetName: "nodes", TargetFormat: tt.format,
			})
			require.NoError(t, err)
			out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: tt.id})
			require.NoError(t, err)
			requireInlineShareFilename(t, out.Headers, tt.expected)
		})
	}

	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "custom-type-share", Name: "custom.txt", TargetKind: "subscription", TargetName: "nodes",
		TargetFormat: "mihomo-proxies", ContentType: "application/x-subscription",
	})
	require.NoError(t, err)
	customTypeOut, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "custom-type-share"})
	require.NoError(t, err)
	require.Equal(t, "application/x-subscription", customTypeOut.ContentType)
	requireInlineShareFilename(t, customTypeOut.Headers, "custom.yaml")
}

func TestServiceRenderSubscriptionShareSanitizesSuggestedFilename(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	t.Run("unicode round trips", func(t *testing.T) {
		_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
			ID: "unicode-share", Name: "配置 🌐.yaml", TargetKind: "subscription", TargetName: "nodes",
		})
		require.NoError(t, err)
		out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "unicode-share", Format: "json-nodes"})
		require.NoError(t, err)
		requireInlineShareFilename(t, out.Headers, "配置 🌐.json")
	})

	t.Run("invalid UTF-8 is replaced", func(t *testing.T) {
		_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
			ID: "utf8-share", Name: string([]byte{'b', 'a', 'd', 0xff, 'n', 'a', 'm', 'e'}), TargetKind: "subscription", TargetName: "nodes",
		})
		require.NoError(t, err)
		out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "utf8-share"})
		require.NoError(t, err)
		requireInlineShareFilename(t, out.Headers, "bad�name.txt")
	})

	t.Run("hostile and control characters are replaced", func(t *testing.T) {
		_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
			ID: "hostile-share", Name: "bad\r\n<>:\"/\\|?*name.yaml", TargetKind: "subscription", TargetName: "nodes",
		})
		require.NoError(t, err)
		out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "hostile-share", Format: "sing-box-outbounds"})
		require.NoError(t, err)
		filename := requireInlineShareFilename(t, out.Headers, "bad___________name.json")
		require.NotContains(t, out.Headers["Content-Disposition"], "\r")
		require.NotContains(t, out.Headers["Content-Disposition"], "\n")
		require.NotContains(t, filename, "/")
		require.NotContains(t, filename, "\\")
	})

	t.Run("overlong basename is bounded before extension", func(t *testing.T) {
		_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
			ID: "long-share", Name: strings.Repeat("界", 150) + ".yaml", TargetKind: "subscription", TargetName: "nodes",
		})
		require.NoError(t, err)
		out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "long-share", Format: "json-nodes"})
		require.NoError(t, err)
		filename := parseInlineShareFilename(t, out.Headers)
		require.True(t, strings.HasSuffix(filename, ".json"))
		require.Equal(t, 128, utf8.RuneCountInString(strings.TrimSuffix(filename, ".json")))
	})

	t.Run("empty sanitized preferred name falls back", func(t *testing.T) {
		_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
			ID: "fallback-share", Name: "...", TargetKind: "subscription", TargetName: "nodes",
		})
		require.NoError(t, err)
		out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "fallback-share"})
		require.NoError(t, err)
		requireInlineShareFilename(t, out.Headers, "nodes.txt")
	})
}

func TestServiceRenderSubscriptionShareAgeFilenameAndOriginalContentType(t *testing.T) {
	ctx := context.Background()
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "encrypted-subscription", Name: "mobile.yaml.age", TargetKind: "subscription", TargetName: "nodes",
		TargetFormat: "mihomo-proxies", AgeRecipient: identity.Recipient().String(),
	})
	require.NoError(t, err)

	out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "encrypted-subscription"})
	require.NoError(t, err)
	require.Equal(t, "application/age", out.ContentType)
	require.Equal(t, "application/yaml", out.Headers["X-Sandrone-Original-Content-Type"])
	filename := requireInlineShareFilename(t, out.Headers, "mobile.yaml.age")
	require.Equal(t, 1, strings.Count(strings.ToLower(filename), ".age"))
}

func TestServiceRenderShareRejectsUnknownFormat(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "nodes", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID: "subscription-share", TargetKind: "subscription", TargetName: "nodes",
	})
	require.NoError(t, err)

	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "subscription-share", Format: "unknown"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))

	out, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "subscription-share"})
	require.NoError(t, err)
	requireInlineShareFilename(t, out.Headers, "nodes.txt")
}

func TestServiceRenderShareHonorsValidTimeRange(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	svc := service.New(service.WithFS(afero.NewMemMapFs()), service.WithClock(func() time.Time { return now }))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "client",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "ok: true\n"},
	}))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "future",
		TargetKind: "file",
		TargetName: "client",
		ValidFrom:  now.Add(time.Minute),
	})
	require.NoError(t, err)
	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "future"})
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "expired",
		TargetKind: "file",
		TargetName: "client",
		ValidUntil: now,
	})
	require.NoError(t, err)
	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "expired"})
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{
		ID:         "active",
		TargetKind: "file",
		TargetName: "client",
		ValidFrom:  now.Add(-time.Minute),
		ValidUntil: now.Add(time.Minute),
	})
	require.NoError(t, err)
	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "active"})
	require.NoError(t, err)
	require.Equal(t, "ok: true\n", string(rendered.Body))
}

func TestServiceDeleteSharePhysicallyRemovesShare(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "client",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "ok: true\n"},
	}))
	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{ID: "delete-me", TargetKind: "file", TargetName: "client"})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteShare(ctx, "delete-me"))
	_, err = svc.GetShare(ctx, "delete-me")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "delete-me"})
	require.ErrorIs(t, err, os.ErrNotExist)

	shares, err := svc.ListShares(ctx)
	require.NoError(t, err)
	require.Empty(t, shares.Shares)
}

func TestServiceCreateShareRejectsUnsupportedTarget(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{ID: "invalid", TargetKind: "unsupported", TargetName: "invalid"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestServiceCreateShareRejectsInvalidAgeRecipient(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{Name: "client", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "ok"}}))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{ID: "invalid-age", TargetKind: "file", TargetName: "client", AgeRecipient: "not-an-age-key"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestServiceCreateShareOverwritesDuplicateID(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{Name: "client", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "ok"}}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{Name: "replacement", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "new"}}))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{ID: "same", Name: "original", TargetKind: "file", TargetName: "client"})
	require.NoError(t, err)
	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{ID: "same", Name: "updated", TargetKind: "file", TargetName: "replacement"})
	require.NoError(t, err)

	stored, err := svc.GetShare(ctx, "same")
	require.NoError(t, err)
	require.Equal(t, "updated", stored.Name)
	require.Equal(t, "replacement", stored.TargetName)
	rendered, err := svc.RenderShare(ctx, domain.ShareRenderRequest{ID: "same"})
	require.NoError(t, err)
	require.Equal(t, "new", string(rendered.Body))
}

func TestServiceCreateShareRejectsMissingTarget(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	_, err := svc.CreateShare(ctx, domain.ShareCreateRequest{
		TargetKind: "file",
		TargetName: "exports/missing.yaml",
	})
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	_, err = svc.CreateShare(ctx, domain.ShareCreateRequest{
		TargetKind: "subscription",
		TargetName: "subscriptions/missing",
	})
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func requireInlineShareFilename(t *testing.T, headers map[string]string, expected string) string {
	t.Helper()
	filename := parseInlineShareFilename(t, headers)
	require.Equal(t, expected, filename)
	return filename
}

func parseInlineShareFilename(t *testing.T, headers map[string]string) string {
	t.Helper()
	disposition := headers["Content-Disposition"]
	require.NotEmpty(t, disposition)
	mediaType, params, err := mime.ParseMediaType(disposition)
	require.NoError(t, err)
	require.Equal(t, "inline", mediaType)
	require.NotEmpty(t, params["filename"])
	return params["filename"]
}
