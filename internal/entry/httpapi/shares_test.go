package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/metacubex/age"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestShareManagementAndPublicEndpoint(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "proxies: []\n"},
	}))
	server := httpapi.New(rt)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/shares", bytes.NewBufferString(`{
		"name": "default mihomo",
		"target_kind": "file",
		"target_name": "default.yaml",
		"content_type": "application/yaml",
		"meta": {"target": "mihomo"}
	}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, createReq)
	require.Equal(t, http.StatusCreated, w.Code)
	for _, field := range []string{`"valid_from"`, `"valid_until"`, `"last_accessed_at"`} {
		require.NotContains(t, w.Body.String(), field)
	}
	var createResult struct {
		Share domain.Share `json:"share"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResult))
	require.NotEmpty(t, createResult.Share.ID)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/shares", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), createResult.Share.ID)
	for _, field := range []string{`"valid_from"`, `"valid_until"`, `"last_accessed_at"`} {
		require.NotContains(t, w.Body.String(), field)
	}

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/shares/"+createResult.Share.ID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"target_name": "default.yaml"`)
	for _, field := range []string{`"valid_from"`, `"valid_until"`, `"last_accessed_at"`} {
		require.NotContains(t, w.Body.String(), field)
	}

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/"+createResult.Share.ID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/yaml", w.Header().Get("Content-Type"))
	requireInlineHTTPFilename(t, w.Header(), "default.yaml")
	require.Equal(t, "proxies: []\n", w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/shares/"+createResult.Share.ID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), `"valid_from"`)
	require.NotContains(t, w.Body.String(), `"valid_until"`)
	require.Contains(t, w.Body.String(), `"last_accessed_at"`)
	var accessedResult struct {
		Share domain.Share `json:"share"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accessedResult))
	require.False(t, accessedResult.Share.LastAccessedAt.IsZero())

	w = httptest.NewRecorder()
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/shares/"+createResult.Share.ID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok": true}`, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/"+createResult.Share.ID, nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPCreateAndRenderSubscriptionShareWithFormatOverride(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	server := httpapi.New(rt)

	createBody := `{"id":"nodes-share","name":"nodes-share","target_kind":"subscription","target_name":"nodes","target_format":"uri-list"}`
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/shares", bytes.NewBufferString(createBody)))
	require.Equal(t, http.StatusCreated, w.Code)
	require.Contains(t, w.Body.String(), `"target_kind": "subscription"`)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/nodes-share?format=mihomo-proxies", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/yaml")
	requireInlineHTTPFilename(t, w.Header(), "nodes-share.yaml")
	require.Contains(t, w.Body.String(), "proxies:")
	require.Contains(t, w.Body.String(), "node-a")
}

func TestHTTPPublicShareReturnsAgePayloadAndEnforcesMaxUses(t *testing.T) {
	ctx := context.Background()
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "secret", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "secret"}}))
	_, err = rt.Service.CreateShare(ctx, domain.ShareCreateRequest{ID: "encrypted", TargetKind: "file", TargetName: "secret", AgeRecipient: identity.Recipient().String(), MaxUses: 1})
	require.NoError(t, err)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/encrypted", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/age", w.Header().Get("Content-Type"))
	require.Equal(t, "application/octet-stream", w.Header().Get("X-Sandrone-Original-Content-Type"))
	requireInlineHTTPFilename(t, w.Header(), "secret.age")
	reader, err := age.Decrypt(bytes.NewReader(w.Body.Bytes()), identity)
	require.NoError(t, err)
	plain, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "secret", string(plain))

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/encrypted", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPPublicSharePassesRequestArgs(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name:   "args.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "hello"},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource("function main(input) { input.file.content = input.file.content + ':' + input.args.name; return input; }"),
			}),
		}},
	}))
	_, err := rt.Service.CreateShare(ctx, domain.ShareCreateRequest{ID: "file-share", TargetKind: "file", TargetName: "args.txt"})
	require.NoError(t, err)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/file-share?format=json-nodes&arg.name=alice", nil))
	require.Equal(t, http.StatusOK, w.Code)
	requireInlineHTTPFilename(t, w.Header(), "args.txt")
	require.Equal(t, "hello:alice", w.Body.String())
}

func TestHTTPDeleteSharePhysicallyDeletesPublicShare(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name:   "client",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "ok: true\n"},
	}))
	_, err := rt.Service.CreateShare(ctx, domain.ShareCreateRequest{ID: "client-share", TargetKind: "file", TargetName: "client"})
	require.NoError(t, err)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/shares/client-share", nil))
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/client-share", nil))
	require.Equal(t, http.StatusNotFound, w.Code)

	_, err = rt.Service.GetShare(ctx, "client-share")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPublicShareEndpointDoesNotRequireBearerToken(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name:   "public.json",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: `[{"name":"a"}]`},
	}))
	share, err := rt.Service.CreateShare(ctx, domain.ShareCreateRequest{
		Name:        "snapshot",
		TargetKind:  "file",
		TargetName:  "public.json",
		ContentType: "application/json",
	})
	require.NoError(t, err)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/"+share.ID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, `[{"name":"a"}]`, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/shares", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/s/"+share.ID, nil))
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/s/"+share.ID, nil))
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func requireInlineHTTPFilename(t *testing.T, header http.Header, expected string) {
	t.Helper()
	disposition := header.Get("Content-Disposition")
	require.NotEmpty(t, disposition)
	mediaType, params, err := mime.ParseMediaType(disposition)
	require.NoError(t, err)
	require.Equal(t, "inline", mediaType)
	require.Equal(t, expected, params["filename"])
}
