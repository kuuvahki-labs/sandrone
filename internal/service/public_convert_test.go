package service_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServicePublicConvertFetchesRemoteSubscriptionSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	}))
	defer server.Close()

	publicFetcher := fetcher.New(
		fetcher.WithResolver(staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}),
		fetcher.WithDialContext(func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}),
	)
	svc := service.New(service.WithFetcher(publicFetcher))

	result, err := svc.ConvertPublic(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: "http://subscription.example/sub"},
	})

	require.NoError(t, err)
	require.Contains(t, string(result.Body), "remote-node")
	require.NotEmpty(t, result.Report.SourceRefs)
	require.Equal(t, "http://subscription.example/sub", result.Report.SourceRefs[0].URL)
}

func TestServicePublicConvertRejectsPrivateRemoteSubscription(t *testing.T) {
	svc := service.New()

	_, err := svc.ConvertPublic(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: "http://127.0.0.1/sub"},
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestServicePublicConvertRejectsRemoteOverrides(t *testing.T) {
	svc := service.New()

	_, err := svc.ConvertPublic(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote: &domain.RemoteInput{
			URL:       "https://subscription.example/sub",
			UserAgent: "custom",
		},
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

type staticResolver struct {
	addresses []net.IPAddr
}

func (r staticResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return r.addresses, nil
}
