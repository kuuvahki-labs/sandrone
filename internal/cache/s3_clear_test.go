package cache_test

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

// Exercise the real SDK and Cache.Clear path, counting actual HTTP operations.
func TestS3ClearUsesPrefixListAndOneBulkRequest(t *testing.T) {
	var mu sync.Mutex
	var operations []string
	var deleted []string
	var requestErr string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		operations = append(operations, r.Method)
		w.Header().Set("Content-Type", "application/xml")
		if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
			if r.URL.Query().Get("prefix") != "ns/cache/" {
				requestErr = "wrong prefix"
			}
			var body strings.Builder
			body.WriteString(`<ListBucketResult><IsTruncated>false</IsTruncated>`)
			for i := range 100 {
				fmt.Fprintf(&body, `<Contents><Key>ns/cache/nested/%03d.json</Key><Size>1</Size><ETag>v1</ETag></Contents>`, i)
			}
			body.WriteString(`</ListBucketResult>`)
			_, _ = w.Write([]byte(body.String()))
			return
		}
		if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
			var payload struct {
				Objects []struct{ Key string } `xml:"Object"`
			}
			if err := xml.NewDecoder(r.Body).Decode(&payload); err != nil {
				requestErr = err.Error()
			}
			for _, object := range payload.Objects {
				deleted = append(deleted, object.Key)
			}
			_, _ = w.Write([]byte(`<DeleteResult/>`))
			return
		}
		requestErr = "unexpected " + r.Method
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	st, err := store.NewS3Store(t.Context(), store.S3Config{Endpoint: server.URL, Region: "auto", Bucket: "bucket", Prefix: "ns/", AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true})
	require.NoError(t, err)
	require.NoError(t, cache.New(st, nil).Clear(t.Context()))
	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, requestErr)
	require.Equal(t, []string{"GET", "POST"}, operations)
	require.Len(t, deleted, 100)
	for _, key := range deleted {
		require.True(t, strings.HasPrefix(key, "ns/cache/nested/"))
	}
}
