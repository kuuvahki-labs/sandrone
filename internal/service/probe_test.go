package service_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestServiceProbeInlineNodes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)

	svc := service.New()
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local",
				Server: "127.0.0.1",
				Port:   uint16(addr.Port),
			}},
		},
		Method:    domain.ProbeTCPConnect,
		TimeoutMS: 1000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.NotNil(t, result.Report.Probe)
	require.Equal(t, 1, result.Report.Probe.SuccessCount)
}

func TestServiceProbeRejectsInvalidTypedNodesBeforeBackend(t *testing.T) {
	t.Parallel()

	called := false
	svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(context.Context, domain.ProbeRequest, []domain.NodeIR, ...probe.Payload) (*domain.ProbeResult, error) {
		called = true
		return &domain.ProbeResult{}, nil
	}}))
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{{
			Name: "invalid", Type: domain.NodeTypeShadowsocks,
			Server: "example.com", Port: 443, Password: "secret",
		}}},
		Method: domain.ProbeTCPConnect,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed))
	require.False(t, called)
}
func TestServiceProbeCache(t *testing.T) {
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	calls := 0
	svc := service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time {
			return time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)
		}),
		service.WithProbeEngine(fakeProbeEngine{probe: func(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
			calls++
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "n",
					Method:     string(req.Method),
					Alive:      true,
					DurationMS: 7,
					CheckedAt:  time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
				}},
				Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake", Method: string(req.Method), SuccessCount: 1}},
			}, nil
		}}),
	)
	req := domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:     "n",
				Type:     domain.NodeTypeShadowsocks,
				Server:   "127.0.0.1",
				Port:     80,
				Cipher:   "aes-128-gcm",
				Password: "p",
			}},
		},
		Method:          domain.ProbeTCPConnect,
		CacheTTLSeconds: 60,
	}
	first, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.False(t, first.Results[0].CacheHit)
	second, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.True(t, second.Results[0].CacheHit)
	require.Equal(t, 1, calls)
}

func TestServiceProbeUsesRuntimeDefaultsBeforeCache(t *testing.T) {
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	seen := []domain.ProbeRequest{}
	svc := service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time {
			return time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)
		}),
		service.WithProbeEngine(fakeProbeEngine{probe: func(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
			seen = append(seen, req)
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "n",
					Method:     string(req.Method),
					Core:       req.Core,
					Alive:      true,
					DurationMS: 7,
					CheckedAt:  time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
				}},
				Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake", Method: string(req.Method), Core: req.Core, SuccessCount: 1}},
			}, nil
		}}),
	)
	putProjectSettings(t, svc, context.Background(), func(update *domain.SettingsUpdate) {
		update.ProbeDefaults = domain.ProbeDefaults{
			Method:          "tcp_connect",
			Core:            "mihomo",
			TimeoutMS:       777,
			Attempts:        3,
			Concurrency:     4,
			CacheTTLSeconds: 60,
		}
	})
	req := domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:     "n",
				Type:     domain.NodeTypeShadowsocks,
				Server:   "127.0.0.1",
				Port:     80,
				Cipher:   "aes-128-gcm",
				Password: "p",
			}},
		},
	}
	first, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.False(t, first.Results[0].CacheHit)
	second, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.True(t, second.Results[0].CacheHit)

	require.Len(t, seen, 1)
	require.Equal(t, domain.ProbeTCPConnect, seen[0].Method)
	require.Empty(t, seen[0].Core)
	require.Equal(t, 777, seen[0].TimeoutMS)
	require.Equal(t, 3, seen[0].Attempts)
	require.Equal(t, 4, seen[0].Concurrency)
	require.Equal(t, 60, seen[0].CacheTTLSeconds)
}

func TestServiceProbeCacheKeySeparatesHealthCheckTargets(t *testing.T) {
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	var seen []domain.ProbeRequest
	svc := service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time {
			return time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)
		}),
		service.WithProbeEngine(fakeProbeEngine{probe: func(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
			seen = append(seen, req)
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "n",
					Method:     string(req.Method),
					Core:       req.Core,
					Target:     req.URL + req.NTPServer,
					Alive:      true,
					DurationMS: len(seen),
					CheckedAt:  time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
				}},
				Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake", Method: string(req.Method), Core: req.Core, SuccessCount: 1}},
			}, nil
		}}),
	)
	base := domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:     "n",
				Type:     domain.NodeTypeShadowsocks,
				Server:   "127.0.0.1",
				Port:     80,
				Cipher:   "aes-128-gcm",
				Password: "p",
			}},
		},
		Method:          domain.ProbeTCPConnect,
		CacheTTLSeconds: 60,
	}

	requests := []domain.ProbeRequest{
		base,
		withProbeMethod(base, domain.ProbeUDPNTP),
		withProbeNTPServer(withProbeMethod(base, domain.ProbeUDPNTP), "time.cloudflare.com"),
		withProbeMethodURL(base, domain.ProbeURLTest, "https://example.com/generate_204"),
		withProbeMethodURL(base, domain.ProbeURLTest, "https://example.net/generate_204"),
		withProbeCore(withProbeMethodURL(base, domain.ProbeURLTest, "https://example.net/generate_204"), "sing-box"),
	}
	for _, req := range requests {
		_, err := svc.Probe(context.Background(), req)
		require.NoError(t, err)
	}

	require.Len(t, seen, len(requests)-1)
}

func withProbeMethod(req domain.ProbeRequest, method domain.ProbeMethod) domain.ProbeRequest {
	req.Method = method
	return req
}

func withProbeNTPServer(req domain.ProbeRequest, server string) domain.ProbeRequest {
	req.NTPServer = server
	return req
}

func withProbeMethodURL(req domain.ProbeRequest, method domain.ProbeMethod, url string) domain.ProbeRequest {
	req.Method = method
	req.URL = url
	return req
}

func withProbeCore(req domain.ProbeRequest, core string) domain.ProbeRequest {
	req.Core = core
	return req
}
