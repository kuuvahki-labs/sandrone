package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
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

func TestServiceProbeNormalizesInlineHysteriaBandwidthWithoutMutatingInput(t *testing.T) {
	for _, inputType := range []string{"inline_nodes", "inline"} {
		t.Run(inputType, func(t *testing.T) {
			callerNodes := []domain.NodeIR{{
				Name: "inline", Type: domain.NodeTypeHysteria, Server: "inline.example", Port: 8443,
				TLS:      &domain.TLSOptions{Enabled: true},
				Hysteria: &domain.HysteriaOptions{Up: "55", Down: "100"},
				Raw:      map[string]json.RawMessage{"caller": json.RawMessage(`"value"`)},
			}}
			var captured domain.HysteriaOptions
			svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, _ domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
				require.Len(t, nodes, 1)
				captured = *nodes[0].Hysteria
				nodes[0].Name = "mutated by engine"
				nodes[0].Hysteria.UpMbps = 999
				nodes[0].Raw["caller"][1] = 'X'
				nodes[0].Raw["engine"] = json.RawMessage(`true`)
				return &domain.ProbeResult{Results: []domain.NodeProbeResult{{Alive: true}}}, nil
			}}))

			result, err := svc.Probe(context.Background(), domain.ProbeRequest{
				Input:  domain.NodeInput{Type: inputType, Nodes: callerNodes},
				Method: domain.ProbeTCPConnect,
			})

			require.NoError(t, err)
			require.Equal(t, domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, captured)
			require.Len(t, result.Report.Warnings, 2)
			for _, warning := range result.Report.Warnings {
				require.Equal(t, "parse_implicit_bandwidth_unit", warning.Code)
			}
			require.Equal(t, "inline", callerNodes[0].Name)
			require.Equal(t, &domain.HysteriaOptions{Up: "55", Down: "100"}, callerNodes[0].Hysteria)
			require.JSONEq(t, `"value"`, string(callerNodes[0].Raw["caller"]))
			require.NotContains(t, callerNodes[0].Raw, "engine")
		})
	}
}

func TestServiceProbeSilentlyCanonicalizesInlineVMessAndVLESSUserIDs(t *testing.T) {
	callerNodes := []domain.NodeIR{
		{Name: "vmess", Type: domain.NodeTypeVMess, Server: "vmess.example", Port: 443, UUID: "123456", Cipher: "auto"},
		{Name: "vless", Type: domain.NodeTypeVLESS, Server: "vless.example", Port: 443, UUID: "a9dk23bz0", Encryption: "none"},
	}
	svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, _ domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
		require.Equal(t, "f8598425-92f2-5508-a071-4fc67f9040ac", nodes[0].UUID)
		require.Equal(t, "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5", nodes[1].UUID)
		return &domain.ProbeResult{Results: []domain.NodeProbeResult{{NodeName: "vmess"}, {NodeName: "vless"}}}, nil
	}}))

	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input:  domain.NodeInput{Type: "inline_nodes", Nodes: callerNodes},
		Method: domain.ProbeTCPConnect,
	})

	require.NoError(t, err)
	require.Empty(t, result.Report.Warnings)
	require.Equal(t, "123456", callerNodes[0].UUID)
	require.Equal(t, "a9dk23bz0", callerNodes[1].UUID)
}

func TestServiceProbeNormalizesInlineHysteriaOverBoundMbps(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	if max == int(^uint(0)>>1) {
		t.Skip("max+1 is not representable as int on this platform")
	}
	callerNodes := []domain.NodeIR{{
		Name: "inline", Type: domain.NodeTypeHysteria, Server: "inline.example", Port: 8443,
		TLS:      &domain.TLSOptions{Enabled: true},
		Hysteria: &domain.HysteriaOptions{Up: "20", UpMbps: max + 1, DownMbps: max},
	}}
	var captured domain.NodeIR
	svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, _ domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
		require.Len(t, nodes, 1)
		captured = nodes[0]
		return &domain.ProbeResult{Results: []domain.NodeProbeResult{{Alive: true}}}, nil
	}}))

	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: callerNodes}, Method: domain.ProbeTCPConnect,
	})

	require.NoError(t, err)
	require.Equal(t, 20, captured.Hysteria.UpMbps)
	require.Equal(t, max, captured.Hysteria.DownMbps)
	require.JSONEq(t, fmt.Sprint(max+1), string(captured.Raw["json-nodes.hysteria.up"]))
	require.Equal(t, []string{"parse_implicit_bandwidth_unit", "parse_unknown_field"}, warningCodes(result.Report.Warnings))
	require.Equal(t, &domain.HysteriaOptions{Up: "20", UpMbps: max + 1, DownMbps: max}, callerNodes[0].Hysteria)
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

func TestServiceURLTestIsolatesUnsupportedTLSClientFingerprintForEveryCore(t *testing.T) {
	t.Parallel()

	for _, core := range []string{"sing-box", "mihomo"} {
		core := core
		t.Run(core, func(t *testing.T) {
			t.Parallel()
			svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
				require.Equal(t, core, req.Core)
				require.Len(t, nodes, 1)
				require.Equal(t, "valid", nodes[0].Name)
				require.Len(t, payloads, 1)
				require.NotContains(t, string(payloads[0].Body), "unsafe")
				return &domain.ProbeResult{Results: []domain.NodeProbeResult{{NodeName: "valid", Core: core}}}, nil
			}}))

			result, err := svc.Probe(context.Background(), domain.ProbeRequest{
				Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
					{Name: "valid", Type: domain.NodeTypeHTTP, Server: "127.0.0.1", Port: 8080},
					{
						Name: "unsupported", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
						UUID: "11111111-1111-1111-1111-111111111111",
						TLS:  &domain.TLSOptions{Enabled: true, ClientFingerprint: "unsafe"},
					},
				}},
				Method: domain.ProbeURLTest, Core: core, URL: "https://example.com/generate_204",
			})

			require.NoError(t, err)
			require.Len(t, result.Results, 1)
			require.True(t, containsWarning(result.Report.Warnings, "node_validation_dropped", "tls.client_fingerprint"))
		})
	}
}

func containsWarning(warnings []domain.Warning, code, field string) bool {
	for _, warning := range warnings {
		if warning.Code == code && warning.Field == field {
			return true
		}
	}
	return false
}

func TestServiceTransientProbeDoesNotUsePersistentCache(t *testing.T) {
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
	require.False(t, second.Results[0].CacheHit)
	require.Zero(t, second.Report.Probe.CacheHitCount)
	require.Empty(t, second.Report.Warnings)
	require.Equal(t, 2, calls)
}

func TestServiceTransientProbeDoesNotReuseNodesAcrossRequests(t *testing.T) {
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	var calls [][]string
	svc := service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time { return time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC) }),
		service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
			servers := make([]string, len(nodes))
			results := make([]domain.NodeProbeResult, len(nodes))
			for index, node := range nodes {
				servers[index] = node.Server
				results[index] = domain.NodeProbeResult{Method: string(req.Method), Alive: true, DurationMS: index + 1, CheckedAt: time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)}
			}
			calls = append(calls, servers)
			return &domain.ProbeResult{Results: results, Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake"}}}, nil
		}}),
	)
	request := func(nodes []domain.NodeIR) *domain.ProbeResult {
		result, err := svc.Probe(context.Background(), domain.ProbeRequest{
			Input: domain.NodeInput{Type: "inline_nodes", Nodes: nodes}, Method: domain.ProbeTCPConnect, CacheTTLSeconds: 60,
		})
		require.NoError(t, err)
		return result
	}
	base := []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks, Server: "a.example", Port: 1, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "b", Type: domain.NodeTypeShadowsocks, Server: "b.example", Port: 2, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "c", Type: domain.NodeTypeShadowsocks, Server: "c.example", Port: 3, Cipher: "aes-128-gcm", Password: "p"},
	}
	first := request(base)
	require.Equal(t, [][]string{{"a.example", "b.example", "c.example"}}, calls)
	require.Zero(t, first.Report.Probe.CacheHitCount)

	reordered := []domain.NodeIR{
		{Name: "renamed-c", Type: domain.NodeTypeShadowsocks, Server: "c.example", Port: 3, Cipher: "aes-128-gcm", Password: "p", Meta: map[string]string{"probe.alive": "true"}},
		{Name: "renamed-a", Type: domain.NodeTypeShadowsocks, Server: "a.example", Port: 1, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "renamed-b", Type: domain.NodeTypeShadowsocks, Server: "b.example", Port: 2, Cipher: "aes-128-gcm", Password: "p"},
	}
	second := request(reordered)
	require.Len(t, calls, 2)
	require.Zero(t, second.Report.Probe.CacheHitCount)
	require.Equal(t, []string{"renamed-c", "renamed-a", "renamed-b"}, []string{second.Results[0].NodeName, second.Results[1].NodeName, second.Results[2].NodeName})
	require.NotEmpty(t, second.Results[0].RuntimeID)
	require.NotEqual(t, second.Results[0].RuntimeID, second.Results[1].RuntimeID)

	reordered[2].Server = "changed.example"
	third := request(reordered)
	require.Equal(t, [][]string{{"a.example", "b.example", "c.example"}, {"c.example", "a.example", "b.example"}, {"c.example", "a.example", "changed.example"}}, calls)
	require.Zero(t, third.Report.Probe.CacheHitCount)
	require.False(t, third.Results[0].CacheHit)
	require.False(t, third.Results[1].CacheHit)
	require.False(t, third.Results[2].CacheHit)
}

func TestServiceTransientProbeStillNormalizesParametersOutsideEffectiveMethod(t *testing.T) {
	fs := afero.NewMemMapFs()
	calls := 0
	svc := service.New(
		service.WithStore(store.NewFSStore(fs)),
		service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
			calls++
			return &domain.ProbeResult{Results: []domain.NodeProbeResult{{Method: string(req.Method), Alive: true, CheckedAt: time.Now()}}}, nil
		}}),
	)
	node := domain.NodeIR{Name: "one", Type: domain.NodeTypeShadowsocks, Server: "same.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"}
	first, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{node}}, Method: domain.ProbeTCPConnect,
		URL: "https://one.example", NTPServer: "one.example", ExpectedStatus: "204", CacheTTLSeconds: 60,
	})
	require.NoError(t, err)
	require.False(t, first.Results[0].CacheHit)

	second, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{node}}, Method: domain.ProbeTCPConnect,
		URL: "https://two.example", NTPServer: "two.example", ExpectedStatus: "200-299", CacheTTLSeconds: 60,
	})
	require.NoError(t, err)
	require.False(t, second.Results[0].CacheHit)
	require.Equal(t, 2, calls)
}

func TestServiceProbeAssociatesFreshResultsByRuntimeID(t *testing.T) {
	svc := service.New(service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
		return &domain.ProbeResult{Results: []domain.NodeProbeResult{
			{RuntimeID: domain.NodeRuntimeID(nodes[1]), Method: string(req.Method), Alive: true, DurationMS: 22},
			{RuntimeID: domain.NodeRuntimeID(nodes[0]), Method: string(req.Method), Alive: true, DurationMS: 11},
		}}, nil
	}}))
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
			{Name: "one", Type: domain.NodeTypeShadowsocks, Server: "one.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
			{Name: "two", Type: domain.NodeTypeShadowsocks, Server: "two.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
		}},
		Method: domain.ProbeTCPConnect,
	})
	require.NoError(t, err)
	require.Equal(t, 11, result.Results[0].DurationMS)
	require.Equal(t, 22, result.Results[1].DurationMS)
}

func TestServiceProbePartialCacheKeepsAllSkippedMissesNodeLocal(t *testing.T) {
	fs := afero.NewMemMapFs()
	calls := 0
	svc := service.New(
		service.WithStore(store.NewFSStore(fs)),
		service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
			calls++
			results := make([]domain.NodeProbeResult, len(nodes))
			for index := range nodes {
				results[index] = domain.NodeProbeResult{Method: string(req.Method), Core: req.Core, Alive: true, CheckedAt: time.Now()}
			}
			return &domain.ProbeResult{Results: results}, nil
		}}),
	)
	supported := domain.NodeIR{Name: "supported", Type: domain.NodeTypeHTTP, Server: "supported.example", Port: 443}
	request := func(nodes []domain.NodeIR) *domain.ProbeResult {
		require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
			Name: "probe/cache", Type: domain.SubscriptionTypeCollection, Nodes: nodes,
		}))
		result, err := svc.Probe(context.Background(), domain.ProbeRequest{
			Input: domain.NodeInput{Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "probe/cache"}}, Method: domain.ProbeURLTest,
			Core: "sing-box", URL: "https://example.com/generate_204", CacheTTLSeconds: 60,
		})
		require.NoError(t, err)
		return result
	}
	request([]domain.NodeIR{supported})
	unsupported := domain.NodeIR{
		Name: "unsupported", Type: domain.NodeTypeSnell, Server: "unsupported.example", Port: 443,
		Password: "p", Snell: &domain.SnellOptions{Version: 5},
	}
	result := request([]domain.NodeIR{supported, unsupported})
	require.Equal(t, 1, calls)
	require.True(t, result.Results[0].CacheHit)
	require.False(t, result.Results[1].CacheHit)
	require.False(t, result.Results[1].Alive)
	require.Equal(t, string(domain.CodeProbeInvalidTarget), result.Results[1].ErrorCode)
	require.True(t, containsWarning(result.Report.Warnings, "render_node_skipped", string(domain.NodeTypeSnell)))
}

func TestServiceProbeGroupsDuplicateConnectionsWithoutSharingRuntimeID(t *testing.T) {
	fs := afero.NewMemMapFs()
	calls := 0
	svc := service.New(
		service.WithStore(store.NewFSStore(fs)),
		service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
			calls++
			require.Len(t, nodes, 1)
			return &domain.ProbeResult{Results: []domain.NodeProbeResult{{Method: string(req.Method), Alive: true, CheckedAt: time.Now()}}}, nil
		}}),
	)
	req := domain.ProbeRequest{Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
		{Name: "one", Type: domain.NodeTypeShadowsocks, Server: "same.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "two", Type: domain.NodeTypeShadowsocks, Server: "same.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
	}}, Method: domain.ProbeTCPConnect, CacheTTLSeconds: 60}
	first, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, first.Results, 2)
	require.Equal(t, 1, calls)
	require.NotEqual(t, first.Results[0].RuntimeID, first.Results[1].RuntimeID)
	require.False(t, first.Results[0].CacheHit)
	require.False(t, first.Results[1].CacheHit)

	second, err := svc.Probe(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Zero(t, second.Report.Probe.CacheHitCount)
}

func TestServiceTransientProbeUsesRuntimeDefaultsBeforeExecution(t *testing.T) {
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
			Method:      "tcp_connect",
			Core:        "mihomo",
			TimeoutMS:   777,
			Attempts:    3,
			Concurrency: 4,
		}
		update.CacheDefaults.ProbeTTLSeconds = 60
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
	require.False(t, second.Results[0].CacheHit)

	require.Len(t, seen, 2)
	require.Equal(t, domain.ProbeTCPConnect, seen[0].Method)
	require.Empty(t, seen[0].Core)
	require.Equal(t, 777, seen[0].TimeoutMS)
	require.Equal(t, 3, seen[0].Attempts)
	require.Equal(t, 4, seen[0].Concurrency)
	require.Equal(t, 60, seen[0].CacheTTLSeconds)
}

func TestServiceTransientProbeExecutesEveryHealthCheckTarget(t *testing.T) {
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

	require.Len(t, seen, len(requests))
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
