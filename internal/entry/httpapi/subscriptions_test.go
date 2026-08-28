package httpapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestListSubscriptionsEndpoint(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(context.Background(), domain.Subscription{
		Name:        "sub",
		DisplayName: "  Primary Nodes  ",
		Type:        domain.SubscriptionTypeLocal,
		Format:      "uri-list",
		Meta:        map[string]string{"description": "daily\nbackup"},
	}))
	server := httpapi.New(rt)
	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var result domain.ResourceListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Items, 1)
	require.Equal(t, "sub", result.Items[0].Name)
	require.Equal(t, "Primary Nodes", result.Items[0].DisplayName)
	require.Equal(t, "subscription", result.Items[0].Kind)
	require.Equal(t, "local", result.Items[0].Type)
	require.Equal(t, "uri-list", result.Items[0].Format)
	require.Equal(t, "daily\nbackup", result.Items[0].Meta["description"])
}

func TestHTTPPublicResourceNamesRejectSlash(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{Name: "remote/provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list"}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "files/default.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{Name: "default.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))
	server := httpapi.New(rt)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/subscriptions/remote%2Fprovider", ""},
		{http.MethodGet, "/v1/subscriptions/remote/provider", ""},
		{http.MethodPost, "/v1/subscriptions/remote%2Fprovider/preview", ""},
		{http.MethodPost, "/v1/subscriptions/remote/provider/traffic", `{}`},
		{http.MethodDelete, "/v1/subscriptions/remote%2Fprovider", ""},
		{http.MethodGet, "/v1/files/files%2Fdefault.yaml?mode=spec", ""},
		{http.MethodDelete, "/v1/files/files/default.yaml", ""},
		{http.MethodGet, "/v1/shares/share%2Fid", ""},
		{http.MethodDelete, "/v1/shares/share%2Fid", ""},
		{http.MethodGet, "/s/share%2Fid", ""},
		{http.MethodPost, "/v1/subscriptions", `{"name":"remote/provider","type":"local","format":"uri-list"}`},
		{http.MethodPost, "/v1/files", `{"name":"files/default.yaml","kind":"static","source":{"type":"inline","content":"body"}}`},
		{http.MethodPost, "/v1/shares", `{"id":"share/id","target_kind":"file","target_name":"default.yaml"}`},
		{http.MethodPost, "/v1/shares", `{"name":"share/name","target_kind":"file","target_name":"default.yaml"}`},
		{http.MethodPost, "/v1/shares", `{"name":"share","target_kind":"file","target_name":"files/default.yaml"}`},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			}
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, body))

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
		})
	}
}

func TestSubscriptionTrafficExposeTrafficAndPreviewOmitsTraffic(t *testing.T) {
	ctx := context.Background()
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#node-a"))
	}))
	defer subServer.Close()

	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:   "provider",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL},
	}))
	server := httpapi.New(rt)

	preview := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/preview", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, preview)
	require.Equal(t, http.StatusOK, w.Code)
	var previewBody map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewBody))
	require.NotContains(t, previewBody, "traffic")
	require.Contains(t, previewBody, "warnings")

	traffic := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/traffic", bytes.NewBufferString(`{"refresh": true}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, traffic)
	require.Equal(t, http.StatusOK, w.Code)
	var trafficBody map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &trafficBody))
	require.Contains(t, trafficBody, "traffic")
	trafficInfo, ok := trafficBody["traffic"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(3072), trafficInfo["used_bytes"])
}

func TestSubscriptionEndpointsUseSingleSegmentNamesForTrafficAndPreview(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	putSubscription := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewBufferString(`{
		"name": "provider",
		"display_name": "  Provider Nodes  ",
		"type": "local",
		"format": "uri-list",
		"content": "ss://aes-128-gcm:secret@example.com:8388#node-a",
		"processors": [
			{"name": "入口重命名", "type": "rename", "stage": "nodes", "params": {"mode": "prefix", "value": "sub-"}}
		],
		"meta": {"description": "daily\nprivate", "origin": "local"}
	}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, putSubscription)
	require.Equal(t, http.StatusCreated, w.Code)

	getSubscription := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/provider", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, getSubscription)
	require.Equal(t, http.StatusOK, w.Code)
	var subscription domain.Subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscription))
	require.Equal(t, "provider", subscription.Name)
	require.Equal(t, "Provider Nodes", subscription.DisplayName)
	require.Equal(t, domain.SubscriptionTypeLocal, subscription.Type)
	require.Equal(t, "daily\nprivate", subscription.Meta["description"])
	require.Len(t, subscription.Processors, 1)
	require.Equal(t, "入口重命名", subscription.Processors[0].Name)

	preview := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/preview", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, preview)
	require.Equal(t, http.StatusOK, w.Code)
	var previewResult domain.SubscriptionPreviewResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewResult))
	require.Equal(t, "provider", previewResult.SubscriptionName)
	require.Equal(t, 1, previewResult.BeforeCount)
	require.Equal(t, 1, previewResult.AfterCount)
	require.Equal(t, "modified", previewResult.Nodes[0].Status)
	require.Equal(t, "node-a", previewResult.Nodes[0].Before.Name)
	require.Equal(t, "sub-node-a", previewResult.Nodes[0].After.Name)
	require.Equal(t, "sub-node-a", previewResult.Nodes[0].TargetNames["shadowrocket"])
	var previewBody map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewBody))
	require.Contains(t, previewBody, "warnings")

	traffic := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/traffic", bytes.NewBufferString(`{"refresh": true}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, traffic)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
	require.Contains(t, w.Body.String(), "subscription traffic requires remote subscription")

	refresh := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/refresh", bytes.NewBufferString(`{}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, refresh)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "/preview or /traffic")

	listSubscriptions := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, listSubscriptions)
	require.Equal(t, http.StatusOK, w.Code)
	var subscriptions domain.ResourceListResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscriptions))
	require.Len(t, subscriptions.Items, 1)
	require.Empty(t, subscriptions.Items[0].Meta["node_count"])

}

func TestSubscriptionActionEndpointsPassRequestArgs(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  if (input.args.keep && input.args.keep !== "yes") {
    input.nodes = [];
  }
  var prefix = input.args.prefix || "";
  input.nodes.forEach(function(node) { node.name = prefix + node.name; });
  return input;
}`),
			}),
		}},
	}))
	server := httpapi.New(rt)

	preview := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/preview?arg.prefix=query-", bytes.NewBufferString(`{"args":{"prefix":"api-"}}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, preview)
	require.Equal(t, http.StatusOK, w.Code)
	var previewResult domain.SubscriptionPreviewResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewResult))
	require.Equal(t, "node-a", previewResult.Nodes[0].Before.Name)
	require.Equal(t, "api-node-a", previewResult.Nodes[0].After.Name)

	traffic := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/traffic", bytes.NewBufferString(`{"refresh": true}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, traffic)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
	require.Contains(t, w.Body.String(), "subscription traffic requires remote subscription")
}

func TestSubscriptionPreviewReturnsSnapshotCacheStatusHeader(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	update := settingsUpdate()
	update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	_, err := rt.Service.PutSettings(ctx, update)
	require.NoError(t, err)
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node",
	}))
	server := httpapi.New(rt)

	request := func(refresh bool) *httptest.ResponseRecorder {
		body := `{}`
		if refresh {
			body = `{"refresh":true}`
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(
			http.MethodPost, "/v1/subscriptions/provider/preview", bytes.NewBufferString(body),
		))
		require.Equal(t, http.StatusOK, response.Code)
		return response
	}

	first := request(false)
	require.Equal(t, "miss", first.Header().Get("X-Sandrone-Subscription-Snapshot"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &body))
	require.NotContains(t, body, "snapshot_cache_status")

	second := request(false)
	require.Equal(t, "hit", second.Header().Get("X-Sandrone-Subscription-Snapshot"))

	refreshed := request(true)
	require.Equal(t, "bypass", refreshed.Header().Get("X-Sandrone-Subscription-Snapshot"))
}

func TestSubscriptionsEndpointUsesPlainTextContent(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	content := "ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#node-a\nvmess://example"
	putSubscription := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewBufferString(`{
		"name": "provider",
		"type": "local",
		"format": "uri-list",
		"content": `+strconv.Quote(content)+`
	}`))
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, putSubscription)

	require.Equal(t, http.StatusCreated, w.Code)
	getSubscription := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/provider", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, getSubscription)
	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, content, response.Content)
}

func TestNamedResourcesAndSubscriptionTrafficEndpoint(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	sub := base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	putRemoteSubscription := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewBufferString(`{
		"name": "provider",
		"type": "remote",
		"format": "base64",
		"remote": {"url": "`+subServer.URL+`"},
		"processors": [
			{"name": "入口重命名", "type": "rename", "stage": "nodes", "params": {"mode": "prefix", "value": "source-"}}
		],
		"meta": {"origin": "remote"}
	}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, putRemoteSubscription)
	require.Equal(t, http.StatusCreated, w.Code)

	getRemoteSubscription := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/provider", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, getRemoteSubscription)
	require.Equal(t, http.StatusOK, w.Code)
	var remoteSubscription domain.Subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &remoteSubscription))
	require.Equal(t, "provider", remoteSubscription.Name)
	require.NotNil(t, remoteSubscription.Remote)
	require.Equal(t, subServer.URL, remoteSubscription.Remote.URL)
	require.Len(t, remoteSubscription.Processors, 1)
	require.Equal(t, "入口重命名", remoteSubscription.Processors[0].Name)
	require.Equal(t, "rename", remoteSubscription.Processors[0].Type)

	putCollection := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewBufferString(`{
		"name": "default",
		"type": "collection",
		"inputs": [
			{"name": "provider", "type": "subscription", "ref": {"kind": "subscription", "name": "provider"}}
		],
		"processors": [
			{"type": "rename", "stage": "nodes", "params": {"mode": "prefix", "value": "live-"}}
		],
		"meta": {"profile": "default"}
	}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, putCollection)
	require.Equal(t, http.StatusCreated, w.Code)

	traffic := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/traffic", bytes.NewBufferString(`{}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, traffic)
	require.Equal(t, http.StatusOK, w.Code)
	var trafficResult map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &trafficResult))
	require.Equal(t, "provider", trafficResult["subscription_name"])
	require.Equal(t, "remote", trafficResult["type"])
	require.Contains(t, trafficResult, "traffic")
	require.IsType(t, map[string]any{}, trafficResult["traffic"])

	collectionTraffic := httptest.NewRequest(http.MethodPost, "/v1/subscriptions/default/traffic", bytes.NewBufferString(`{}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, collectionTraffic)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code": "invalid_argument"`)
	require.Contains(t, w.Body.String(), "subscription traffic requires remote subscription")

	getCollection := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/default", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, getCollection)
	require.Equal(t, http.StatusOK, w.Code)
	var collection domain.Subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &collection))
	require.Equal(t, "default", collection.Name)
	require.Empty(t, collection.Nodes)
	require.Equal(t, "default", collection.Meta["profile"])
}

func TestSubscriptionPreviewEndpointUsesSingleSegmentName(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "rename",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"mode":  "prefix",
				"value": "source-",
			}),
		}},
	}))
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/subscriptions/provider/preview", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var preview domain.SubscriptionPreviewResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &preview))
	require.Equal(t, "provider", preview.SubscriptionName)
	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Equal(t, "modified", preview.Nodes[0].Status)
	require.Equal(t, "node-a", preview.Nodes[0].Before.Name)
	require.Equal(t, "source-node-a", preview.Nodes[0].After.Name)
}

func TestSubscriptionPreviewEndpointAllowsPreviewWordInName(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name:    "preview-provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/subscriptions/preview-provider/preview", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var preview domain.SubscriptionPreviewResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &preview))
	require.Equal(t, "preview-provider", preview.SubscriptionName)
	require.Equal(t, map[string]int{"added": 0, "modified": 0, "removed": 0, "unchanged": 1}, preview.StatusCounts)
}
