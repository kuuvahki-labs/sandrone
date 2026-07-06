package fetcher_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
)

func TestFetchRejectsUnsupportedScheme(t *testing.T) {
	_, err := fetcher.New().Fetch(context.Background(), fetcher.Request{URL: "file:///tmp/x"})
	require.Error(t, err)
}

func TestFetchUsesVersionedDefaultUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "sandrone/0.1.0", r.UserAgent())
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	_, err := fetcher.New().Fetch(context.Background(), fetcher.Request{URL: server.URL})
	require.NoError(t, err)
}

func TestFetchUsesDefaultStatusHashAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "sandrone-test", r.UserAgent())
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3")
		_, _ = w.Write([]byte("remote-body"))
	}))
	defer server.Close()
	sum := sha256.Sum256([]byte("remote-body"))
	result, err := fetcher.New().Fetch(context.Background(), fetcher.Request{
		URL:       server.URL,
		UserAgent: "sandrone-test",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("remote-body"), result.Body)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.Equal(t, hex.EncodeToString(sum[:]), result.ContentHash)
	require.Equal(t, "upload=1; download=2; total=3", result.Headers.Get("Subscription-Userinfo"))
}

func TestFetchRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()
	_, err := fetcher.New().Fetch(context.Background(), fetcher.Request{URL: server.URL})
	require.Error(t, err)
}

func TestFetchEnforcesDefaultSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, zeroReader{}, fetcher.DefaultMaxBytes+1)
	}))
	defer server.Close()
	_, err := fetcher.New().Fetch(context.Background(), fetcher.Request{URL: server.URL})
	require.Error(t, err)
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
