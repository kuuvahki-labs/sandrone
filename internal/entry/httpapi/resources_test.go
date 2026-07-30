package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestDeleteResourcesEndpoint(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{Name: "provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list"}))
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{Name: "default", Type: domain.SubscriptionTypeCollection}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "stored-base.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))

	server := httpapi.New(rt)
	cases := []struct {
		name       string
		deletePath string
		getPath    string
	}{
		{name: "subscription source", deletePath: "/v1/subscriptions/provider", getPath: "/v1/subscriptions/provider"},
		{name: "subscription collection", deletePath: "/v1/subscriptions/default", getPath: "/v1/subscriptions/default"},
		{name: "file", deletePath: "/v1/files/stored-base.yaml", getPath: "/v1/files/stored-base.yaml?mode=spec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, tc.deletePath, nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.JSONEq(t, `{"ok": true}`, w.Body.String())

			w = httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.getPath, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}

	for _, path := range []string{"/v1/subscriptions", "/v1/files"} {
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code)
		var result domain.ResourceListResult
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		require.Empty(t, result.Items)
	}
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/shares", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var shares domain.ShareListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &shares))
	require.Empty(t, shares.Shares)
}

func TestFileEndpointStoresCompleteInlineSourceInJSONRecord(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	body := []byte(`{
  "name": "rename.js",
	"kind": "static",
  "display_name": "  Rename Script  ",
  "source": {
    "type": "inline",
    "content": "function main(input) { return input; }\n"
  },
  "processors": [],
  "meta": {"description": "node rename script\nused by file stage"}
}`)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code)

	_, err := os.ReadFile(filepath.Join(rt.Config.DataDir, "files", "rename.js"))
	require.True(t, os.IsNotExist(err))

	metadataBody, err := os.ReadFile(filepath.Join(rt.Config.DataDir, "files", "rename.js.json"))
	require.NoError(t, err)
	require.Contains(t, string(metadataBody), "function main")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/rename.js?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var spec domain.FileSpec
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	require.Equal(t, "rename.js", spec.Name)
	require.Equal(t, "Rename Script", spec.DisplayName)
	require.Equal(t, "node rename script\nused by file stage", spec.Meta["description"])
	require.Equal(t, "inline", spec.Source.Type)
	require.Equal(t, "function main(input) { return input; }\n", spec.Source.Content)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var files domain.ResourceListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &files))
	require.Len(t, files.Items, 1)
	require.Equal(t, "rename.js", files.Items[0].Name)
	require.Equal(t, "Rename Script", files.Items[0].DisplayName)
	require.Equal(t, "node rename script\nused by file stage", files.Items[0].Meta["description"])

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/rename.js", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "function main(input) { return input; }\n", w.Body.String())
}

func TestFileEndpointStoresAndRendersTypedConfigFile(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	server := httpapi.New(rt)

	body := []byte(`{
  "name": "default.yaml",
  "kind": "mihomo",
  "source": {},
  "config": {
    "subscriptions": ["provider"],
	"settings": {
	  "adaptive_groups": {
		"type": "url-test",
		"regions": ["hk", "jp"]
	  }
    }
  }
}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var spec domain.FileSpec
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	require.Equal(t, domain.FileKindMihomo, spec.Kind)
	require.Equal(t, []string{"provider"}, spec.Config.Subscriptions)
	require.JSONEq(t, `{"adaptive_groups":{"type":"url-test","regions":["hk","jp"]}}`, string(spec.Config.Settings))

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/yaml", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "proxy-groups:")
	require.Contains(t, w.Body.String(), "node-a")
	require.NotContains(t, w.Body.String(), "adaptive_groups")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var files domain.ResourceListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &files))
	require.Len(t, files.Items, 1)
	require.Empty(t, files.Items[0].Type)
	require.Equal(t, "mihomo", files.Items[0].Target)
}

func TestFileEndpointRejectsNonCanonicalKindsAndLegacyConfigWire(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing kind", body: `{"name":"bad.yaml","source":{"type":"inline","content":"body"}}`, want: "file kind is required"},
		{name: "case variant", body: `{"name":"bad.yaml","kind":"Mihomo","source":{}}`, want: `file kind \"Mihomo\"`},
		{name: "unknown kind", body: `{"name":"bad.yaml","kind":"future","source":{}}`, want: `file kind \"future\"`},
		{name: "legacy config", body: `{"name":"bad.yaml","kind":"mihomo","source":{},"config":{"groups":[]}}`, want: "invalid JSON body"},
		{name: "null settings", body: `{"name":"bad.yaml","kind":"mihomo","source":{},"config":{"settings":null}}`, want: "config.settings"},
		{name: "unknown settings", body: `{"name":"bad.yaml","kind":"sing-box","source":{},"config":{"settings":{"future":true}}}`, want: "config.settings.future"},
		{name: "empty source with content", body: `{"name":"bad.yaml","kind":"mihomo","source":{"content":"ignored"},"config":{}}`, want: "empty file source must not include content or remote"},
		{name: "empty source with remote", body: `{"name":"bad.yaml","kind":"mihomo","source":{"remote":{"url":"https://example.com/ignored"}},"config":{}}`, want: "empty file source must not include content or remote"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := testRuntime(t, app.Config{})
			server := httpapi.New(rt)
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(test.body)))

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
			require.Contains(t, w.Body.String(), test.want)
			files, err := rt.Service.ListFiles(context.Background())
			require.NoError(t, err)
			require.Empty(t, files.Items)
		})
	}
}

func TestFileEndpointStoresAndRendersSingBoxWebDefault(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	server := httpapi.New(rt)

	body := []byte(`{
  "name": "default.json",
  "kind": "sing-box",
  "source": {
    "type": "inline",
    "content": "{\"outbounds\":[],\"route\":{\"rule_set\":[],\"rules\":[]}}"
  },
  "config": {
    "subscriptions": ["provider"],
	"settings": {
	  "groups": [
		{"type":"selector","tag":"Proxy","outbounds":["Auto","$nodes","direct"]},
		{"type":"urltest","tag":"Auto","outbounds":["$nodes"],"url":"https://www.gstatic.com/generate_204","interval":"5m","tolerance":50}
	  ],
	  "rule_sets": [
		{"type":"inline","tag":"private","rules":[{"domain_suffix":["local"]}]}
	  ],
	  "rules": [
		{"rule_set":["private"],"outbound":"direct"},
		{"ip_is_private":true,"outbound":"direct"},
		{"outbound":"Proxy"}
	  ]
    }
  },
  "processors": [{
    "name": "Sniff & DNS Hijack",
    "type": "merge",
    "stage": "file",
    "params": {
      "mode": "json_override",
      "content": "{\"route\":{\"+rules\":[{\"action\":\"sniff\"},{\"protocol\":\"dns\",\"action\":\"hijack-dns\"}]}}"
    }
  }]
}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.json?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var spec domain.FileSpec
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	require.Equal(t, domain.FileKindSingBox, spec.Kind)
	require.Equal(t, []string{"provider"}, spec.Config.Subscriptions)
	require.JSONEq(t, `"json_override"`, string(spec.Processors[0].Params["mode"]))

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.json", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var rendered struct {
		Outbounds []map[string]any `json:"outbounds"`
		Route     struct {
			Rules []map[string]any `json:"rules"`
		} `json:"route"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rendered))
	require.Equal(t, "Proxy", rendered.Outbounds[0]["tag"])
	tags := make([]string, 0, len(rendered.Outbounds))
	for _, outbound := range rendered.Outbounds {
		tag, _ := outbound["tag"].(string)
		tags = append(tags, tag)
	}
	require.Contains(t, tags, "node-a")
	require.Equal(t, "sniff", rendered.Route.Rules[0]["action"])
	require.Equal(t, "hijack-dns", rendered.Route.Rules[1]["action"])
	require.Equal(t, "Proxy", rendered.Route.Rules[len(rendered.Route.Rules)-1]["outbound"])
}

func TestFileEndpointExposesSourceAsBodyOrJSONWithoutChangingSpec(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: "mixed-port: 7891\nmarker: source\n"},
		Config: &domain.FileConfig{},
	}))
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml?mode=source", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/yaml", w.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "mixed-port: 7891\nmarker: source\n", w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml?mode=source&response=json", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{
  "content_type": "application/yaml",
  "body": "mixed-port: 7891\nmarker: source\n"
}`, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var spec domain.FileSpec
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	require.Equal(t, "inline", spec.Source.Type)
	require.Equal(t, "mixed-port: 7891\nmarker: source\n", spec.Source.Content)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/default.yaml", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "proxy-groups:")
	require.Contains(t, w.Body.String(), "marker: source")
}

func TestDeleteResourcePreservesLiteralPercentSlashName(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "percent%2Fname", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))

	server := httpapi.New(rt)
	const path = "/v1/files/percent%252Fname"

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path+"?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"name": "percent%2Fname"`)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, path, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok": true}`, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path+"?mode=spec", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestFilesNamedConfigAndConfigJSONRenderAndDeleteIndependently(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "config", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "plain"},
	}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "config.json", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "json"},
	}))
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var files domain.ResourceListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &files))
	require.Len(t, files.Items, 2)
	require.Equal(t, []string{"config", "config.json"}, []string{files.Items[0].Name, files.Items[1].Name})

	for path, want := range map[string]string{
		"/v1/files/config":      "plain",
		"/v1/files/config.json": "json",
	} {
		w = httptest.NewRecorder()
		server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, want, w.Body.String())
	}

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/files/config", nil))
	require.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/config.json", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "json", w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/files/config.json", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteResourceRejectsTrailingSlashAlias(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "base.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))

	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/files/base.yaml/", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/base.yaml?mode=spec", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"name": "base.yaml"`)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/files/base.yaml", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok": true}`, w.Body.String())

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/base.yaml?mode=spec", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteResourceRejectsExactRootPaths(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	for _, deletePath := range []string{"/v1/subscriptions", "/v1/files", "/v1/shares"} {
		t.Run(deletePath, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, deletePath, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.NotEqual(t, http.StatusMovedPermanently, w.Code)
			require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
		})
	}
}

func TestDeleteResourceRejectsCleanPathAliases(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "base.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))

	server := httpapi.New(rt)
	for _, deletePath := range []string{
		"/v1/files//base.yaml",
		"/v1/files/./base.yaml",
		"/v1/files/base.yaml/../other.yaml",
	} {
		t.Run(deletePath, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, deletePath, nil))
			require.NotEqual(t, http.StatusMovedPermanently, w.Code)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)

			w = httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/files/base.yaml?mode=spec", nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), `"name": "base.yaml"`)
		})
	}
}

func TestDeleteResourceRejectsEncodedBackslashName(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/files/foo%5Cbar", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
	require.Contains(t, w.Body.String(), "single path segment")
}
