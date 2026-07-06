package service_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSubscriptionTrafficIsRemoteOnlyAndPreviewDetectsCycles(t *testing.T) {
	sub := base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/provider",
		Type:   domain.SubscriptionTypeRemote,
		Format: "base64",
		Remote: &domain.RemoteInput{URL: subServer.URL},
		Processors: []domain.ProcessorSpec{{
			Type:  "rename",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"mode":  "prefix",
				"value": "remote-",
			}),
		}},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "local/raw",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.org:8389#local-node",
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "groups/live",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{
			{Name: "remote/provider", Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "remote/provider"}},
			{Name: "local/raw", Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "local/raw"}},
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "filter",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"action":  "drop",
				"field":   "name",
				"match":   "regex",
				"pattern": "local-node",
			}),
		}},
	}))

	remoteResult, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/provider"})
	require.NoError(t, err)
	require.NotNil(t, remoteResult.Traffic)
	require.Equal(t, "remote/provider", remoteResult.Traffic.SourceName)
	require.Equal(t, int64(3072), remoteResult.Traffic.UsedBytes)

	_, err = svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "local/raw"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription traffic requires remote subscription")

	_, err = svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "groups/live"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription traffic requires remote subscription")

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "groups/a",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{
			Name: "groups/b",
			Type: "subscription",
			Ref:  domain.ResourceRef{Kind: "subscription", Name: "groups/b"},
		}},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "groups/b",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{
			Name: "groups/a",
			Type: "subscription",
			Ref:  domain.ResourceRef{Kind: "subscription", Name: "groups/a"},
		}},
	}))
	_, err = svc.PreviewSubscription(ctx, "groups/a")
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription dependency cycle")
}

func TestServiceProbeProcessorAllowsRuntimeProbeDefaults(t *testing.T) {
	seen := []domain.ProbeRequest{}
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProbeEngine(fakeProbeEngine{probe: func(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
			seen = append(seen, req)
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "node-a",
					Layer:      string(req.Layer),
					Method:     string(req.Method),
					Core:       req.Core,
					Target:     req.URL,
					Alive:      true,
					DurationMS: 7,
					CheckedAt:  time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
				}},
				Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake", Layer: string(req.Layer), Method: string(req.Method), Core: req.Core, SuccessCount: 1}},
			}, nil
		}}),
	)
	require.NoError(t, svc.PutRuntimeSettings(context.Background(), domain.RuntimeSettings{
		ProbeDefaults: domain.ProbeDefaults{
			Core:      "mihomo",
			URL:       "https://probe.example/generate_204",
			TimeoutMS: 8000,
		},
	}))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "probe",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"method": "url_test",
			}),
		}},
	}))

	_, err := svc.PreviewSubscription(context.Background(), "provider")
	require.NoError(t, err)

	require.Len(t, seen, 1)
	require.Equal(t, domain.ProbeURLTest, seen[0].Method)
	require.Equal(t, "mihomo", seen[0].Core)
	require.Equal(t, "https://probe.example/generate_204", seen[0].URL)
	require.Equal(t, 8000, seen[0].TimeoutMS)
}

func TestServiceSubscriptionProcessorsRoundtripAndPreviewOrder(t *testing.T) {
	sub := base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	subscriptionProcessors := []domain.ProcessorSpec{{
		Type:  "rename",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"mode":  "prefix",
			"value": "source-",
		}),
	}}
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:       "remote/provider",
		Type:       domain.SubscriptionTypeRemote,
		Format:     "base64",
		Remote:     &domain.RemoteInput{URL: subServer.URL},
		Processors: subscriptionProcessors,
	}))

	storedSource, err := svc.GetSubscription(ctx, "remote/provider")
	require.NoError(t, err)
	require.Len(t, storedSource.Processors, 1)
	require.Equal(t, subscriptionProcessors[0].Type, storedSource.Processors[0].Type)
	require.Equal(t, subscriptionProcessors[0].Stage, storedSource.Processors[0].Stage)
	require.JSONEq(t, string(subscriptionProcessors[0].Params["mode"]), string(storedSource.Processors[0].Params["mode"]))
	require.JSONEq(t, string(subscriptionProcessors[0].Params["value"]), string(storedSource.Processors[0].Params["value"]))

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "groups/live",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{
			Name: "provider",
			Type: "subscription",
			Ref:  domain.ResourceRef{Kind: "subscription", Name: "remote/provider"},
		}},
		Processors: []domain.ProcessorSpec{{
			Type:  "rename",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"mode":  "prefix",
				"value": "collection-",
			}),
		}},
	}))
	result, err := svc.PreviewSubscription(ctx, "groups/live")
	require.NoError(t, err)
	require.Equal(t, 1, result.BeforeCount)
	require.Equal(t, 1, result.AfterCount)
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "subscription", Name: "remote/provider"})
}

func TestServicePreviewSubscriptionDiffsByConnectionIdentity(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "remote/provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#keep\nss://aes-128-gcm:secret@example.org:8389#drop",
		Processors: []domain.ProcessorSpec{
			{
				Type:  "rename",
				Stage: domain.StageNodes,
				Params: params(t, map[string]any{
					"mode":  "prefix",
					"value": "source-",
				}),
			},
			{
				Type:  "filter",
				Stage: domain.StageNodes,
				Params: params(t, map[string]any{
					"action":  "drop",
					"field":   "name",
					"match":   "regex",
					"pattern": "source-drop",
				}),
			},
		},
	}))

	preview, err := svc.PreviewSubscription(ctx, "remote/provider")
	require.NoError(t, err)

	require.Equal(t, "remote/provider", preview.SubscriptionName)
	require.Equal(t, 2, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Equal(t, map[string]int{"added": 0, "modified": 1, "removed": 1, "unchanged": 0}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 2)
	require.Equal(t, "modified", preview.Nodes[0].Status)
	require.NotEmpty(t, preview.Nodes[0].Identity)
	require.NotContains(t, preview.Nodes[0].Identity, "secret")
	require.Equal(t, "keep", preview.Nodes[0].Before.Name)
	require.Equal(t, "source-keep", preview.Nodes[0].After.Name)
	require.Equal(t, "removed", preview.Nodes[1].Status)
	require.Equal(t, "drop", preview.Nodes[1].Before.Name)
	require.Nil(t, preview.Nodes[1].After)
	require.Equal(t, "subscription_preview", preview.Report.Kind)
}

func TestServicePreviewSubscriptionTreatsEmptyHysteriaRoundtripAsSameIdentity(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "local/hysteria",
		Type:   domain.SubscriptionTypeLocal,
		Format: "uri-list",
		Content: "hysteria2://secret@example.com:443?peer=example.com&sni=example.com&alpn=h3#first\n" +
			"hysteria2://secret@example.org:8443?peer=example.org&sni=example.org&alpn=h3#second",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  input.nodes.forEach(function(node, index) { node.name = "renamed-" + (index + 1); });
  return input;
}`),
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "local/hysteria")
	require.NoError(t, err)

	require.Equal(t, 2, preview.BeforeCount)
	require.Equal(t, 2, preview.AfterCount)
	require.Equal(t, map[string]int{"added": 0, "modified": 2, "removed": 0, "unchanged": 0}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 2)
	require.Equal(t, "modified", preview.Nodes[0].Status)
	require.Equal(t, "first", preview.Nodes[0].Before.Name)
	require.Equal(t, "renamed-1", preview.Nodes[0].After.Name)
	require.Equal(t, preview.Nodes[0].Before.Server, preview.Nodes[0].After.Server)
	require.Equal(t, preview.Nodes[0].Before.Port, preview.Nodes[0].After.Port)
	require.Equal(t, "modified", preview.Nodes[1].Status)
	require.Equal(t, "second", preview.Nodes[1].Before.Name)
	require.Equal(t, "renamed-2", preview.Nodes[1].After.Name)
	require.Equal(t, preview.Nodes[1].Before.Server, preview.Nodes[1].After.Server)
	require.Equal(t, preview.Nodes[1].Before.Port, preview.Nodes[1].After.Port)
}

func TestServicePreviewSubscriptionStillDistinguishesNonEmptyHysteriaIdentity(t *testing.T) {
	ctx := context.Background()
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProcessor(func(r *processor.Registry) {
			r.RegisterNode("set_hysteria_auth", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
				return hysteriaAuthProcessor{}, nil
			})
		}),
	)
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "local/hysteria-auth",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "hysteria2://secret@example.com:443?peer=example.com&sni=example.com&alpn=h3#first",
		Processors: []domain.ProcessorSpec{{
			Type:  "set_hysteria_auth",
			Stage: domain.StageNodes,
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "local/hysteria-auth")
	require.NoError(t, err)

	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Equal(t, map[string]int{"added": 1, "modified": 0, "removed": 1, "unchanged": 0}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 2)
}

func TestServicePreviewSubscriptionDoesNotShiftDuplicateConnectionIdentityAfterRemoval(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/duplicates",
		Type:   domain.SubscriptionTypeLocal,
		Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#drop-me\n" +
			"ss://aes-128-gcm:secret@example.com:8388#keep-one\n" +
			"ss://aes-128-gcm:secret@example.com:8388#keep-two",
		Processors: []domain.ProcessorSpec{{
			Type:  "filter",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"action":  "drop",
				"field":   "name",
				"match":   "regex",
				"pattern": "drop-me",
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "remote/duplicates")
	require.NoError(t, err)

	require.Equal(t, 3, preview.BeforeCount)
	require.Equal(t, 2, preview.AfterCount)
	require.Equal(t, map[string]int{"added": 0, "modified": 0, "removed": 1, "unchanged": 2}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 3)
	require.Equal(t, "removed", preview.Nodes[0].Status)
	require.Equal(t, "drop-me", preview.Nodes[0].Before.Name)
	require.Nil(t, preview.Nodes[0].After)
	require.Equal(t, "unchanged", preview.Nodes[1].Status)
	require.Equal(t, "keep-one", preview.Nodes[1].Before.Name)
	require.Equal(t, "keep-one", preview.Nodes[1].After.Name)
	require.Equal(t, "unchanged", preview.Nodes[2].Status)
	require.Equal(t, "keep-two", preview.Nodes[2].Before.Name)
	require.Equal(t, "keep-two", preview.Nodes[2].After.Name)
}

func TestServicePreviewSubscriptionKeepsInsertedDuplicateConnectionIdentityInPosition(t *testing.T) {
	ctx := context.Background()
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProcessor(func(r *processor.Registry) {
			r.RegisterNode("insert_first", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
				return insertFirstProcessor{}, nil
			})
		}),
	)
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/insert",
		Type:   domain.SubscriptionTypeLocal,
		Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#keep-one\n" +
			"ss://aes-128-gcm:secret@example.com:8388#keep-two",
		Processors: []domain.ProcessorSpec{{
			Type:  "insert_first",
			Stage: domain.StageNodes,
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "remote/insert")
	require.NoError(t, err)

	require.Equal(t, map[string]int{"added": 1, "modified": 0, "removed": 0, "unchanged": 2}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 3)
	require.Equal(t, "added", preview.Nodes[0].Status)
	require.Nil(t, preview.Nodes[0].Before)
	require.Equal(t, "inserted", preview.Nodes[0].After.Name)
	require.Equal(t, "unchanged", preview.Nodes[1].Status)
	require.Equal(t, "keep-one", preview.Nodes[1].Before.Name)
	require.Equal(t, "keep-one", preview.Nodes[1].After.Name)
	require.Equal(t, "unchanged", preview.Nodes[2].Status)
	require.Equal(t, "keep-two", preview.Nodes[2].Before.Name)
	require.Equal(t, "keep-two", preview.Nodes[2].After.Name)
}

func TestServicePreviewSubscriptionSortKeepsNodesUnchanged(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/sort",
		Type:   domain.SubscriptionTypeLocal,
		Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@b.example:8388#b\n" +
			"ss://aes-128-gcm:secret@a.example:8389#a\n" +
			"ss://aes-128-gcm:secret@c.example:8390#c",
		Processors: []domain.ProcessorSpec{{
			Type:  "sort",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"by": "+name",
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "remote/sort")
	require.NoError(t, err)

	require.Equal(t, map[string]int{"added": 0, "modified": 0, "removed": 0, "unchanged": 3}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 3)
	require.Equal(t, []string{"a", "b", "c"}, []string{
		preview.Nodes[0].After.Name,
		preview.Nodes[1].After.Name,
		preview.Nodes[2].After.Name,
	})
	for _, node := range preview.Nodes {
		require.Equal(t, "unchanged", node.Status)
		require.Equal(t, node.Before.Name, node.After.Name)
	}
}

func TestServicePreviewSubscriptionProbeAnnotationKeepsNodesUnchanged(t *testing.T) {
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProbeEngine(fakeProbeEngine{probe: func(context.Context, domain.ProbeRequest, []domain.NodeIR, ...probe.Payload) (*domain.ProbeResult, error) {
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "node-a",
					Layer:      "protocol",
					Method:     "tcp_connect",
					Alive:      true,
					DurationMS: 11,
					CheckedAt:  checkedAt,
				}},
			}, nil
		}}),
	)
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "remote/probe",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "probe",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"method":   "tcp_connect",
				"annotate": true,
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "remote/probe")
	require.NoError(t, err)

	require.Equal(t, map[string]int{"added": 0, "modified": 0, "removed": 0, "unchanged": 1}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "unchanged", preview.Nodes[0].Status)
	require.Equal(t, "node-a", preview.Nodes[0].Before.Name)
	require.Equal(t, "node-a", preview.Nodes[0].After.Name)
	require.Equal(t, "node-a", preview.Nodes[0].TargetNames["shadowrocket"])
}

type insertFirstProcessor struct{}

func (insertFirstProcessor) Name() string { return "insert_first" }

func (insertFirstProcessor) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := append([]domain.NodeIR{}, in.Nodes...)
	inserted := out[0]
	inserted.Name = "inserted"
	out = append([]domain.NodeIR{inserted}, out...)
	return domain.NodeProcessOutput{Nodes: out}, nil
}

type hysteriaAuthProcessor struct{}

func (hysteriaAuthProcessor) Name() string { return "set_hysteria_auth" }

func (hysteriaAuthProcessor) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := append([]domain.NodeIR{}, in.Nodes...)
	for i := range out {
		hysteria := out[i].Hysteria
		if hysteria == nil {
			hysteria = &domain.HysteriaOptions{}
		} else {
			copy := *hysteria
			hysteria = &copy
		}
		hysteria.Auth = "changed"
		out[i].Hysteria = hysteria
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}

func TestServicePreviewSubscriptionWithoutProcessorsLeavesNodesUnchanged(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "local/raw",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	preview, err := svc.PreviewSubscription(ctx, "local/raw")
	require.NoError(t, err)

	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Equal(t, map[string]int{"added": 0, "modified": 0, "removed": 0, "unchanged": 1}, preview.StatusCounts)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "unchanged", preview.Nodes[0].Status)
	require.Equal(t, "node-a", preview.Nodes[0].Before.Name)
	require.Equal(t, "node-a", preview.Nodes[0].After.Name)
}

func TestServicePreviewRemoteSubscriptionFetchesLatestWithoutWritingSnapshot(t *testing.T) {
	ctx := context.Background()
	body := "ss://aes-128-gcm:secret@example.com:8388#first"
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/live",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL},
	}))

	first, err := svc.PreviewSubscription(ctx, "remote/live")
	require.NoError(t, err)
	require.Equal(t, "first", first.Nodes[0].Before.Name)

	body = "ss://aes-128-gcm:secret@example.com:8388#second"
	second, err := svc.PreviewSubscription(ctx, "remote/live")
	require.NoError(t, err)
	require.Equal(t, "second", second.Nodes[0].Before.Name)

	stored, err := svc.GetSubscription(ctx, "remote/live")
	require.NoError(t, err)
	require.Empty(t, stored.Content)
	require.NotNil(t, stored.Remote)
	require.Equal(t, subServer.URL, stored.Remote.URL)
	subscriptions, err := svc.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, subscriptions.Items, 1)
}

func TestServicePreviewRemoteSubscriptionAutoFormatDetectsFetchedContent(t *testing.T) {
	ctx := context.Background()
	body := base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#auto-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "aa",
		Type:   domain.SubscriptionTypeRemote,
		Format: "auto",
		Remote: &domain.RemoteInput{URL: subServer.URL},
		Processors: []domain.ProcessorSpec{{
			Type:  "quick_settings",
			Stage: domain.StageNodes,
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "aa")
	require.NoError(t, err)

	require.Equal(t, "aa", preview.SubscriptionName)
	require.Equal(t, "auto", preview.Format)
	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "auto-node", preview.Nodes[0].Before.Name)
	require.Equal(t, "unchanged", preview.Nodes[0].Status)
}

func TestServiceSubscriptionTrafficIncludesRemoteHeadersAndPreviewStillWorks(t *testing.T) {
	ctx := context.Background()
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240; expire=1780272000; reset_day=14")
		w.Header().Set("Profile-Web-Page-Url", "https://panel.example.test")
		w.Header().Set("Plan-Name", "VIP 1")
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/provider",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL},
	}))

	result, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/provider", Refresh: true})
	require.NoError(t, err)
	require.NotNil(t, result.Traffic)
	traffic := result.Traffic
	require.Equal(t, "remote/provider", traffic.SourceName)
	require.Equal(t, subServer.URL, traffic.SourceURL)
	require.Equal(t, int64(1024), traffic.UploadBytes)
	require.Equal(t, int64(2048), traffic.DownloadBytes)
	require.Equal(t, int64(3072), traffic.UsedBytes)
	require.NotNil(t, traffic.TotalBytes)
	require.Equal(t, int64(10240), *traffic.TotalBytes)
	require.NotNil(t, traffic.RemainingBytes)
	require.Equal(t, int64(7168), *traffic.RemainingBytes)
	require.Equal(t, "2026-06-01T00:00:00Z", traffic.ExpiresAt)
	require.NotNil(t, traffic.ResetDay)
	require.Equal(t, 14, *traffic.ResetDay)
	require.Equal(t, "https://panel.example.test", traffic.AppURL)
	require.Equal(t, "VIP 1", traffic.PlanName)

	preview, err := svc.PreviewSubscription(ctx, "remote/provider")
	require.NoError(t, err)
	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
}

func TestServiceSubscriptionTrafficNoFlowFragmentSuppressesTraffic(t *testing.T) {
	ctx := context.Background()
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/noflow",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL + "#noFlow&token=secret"},
	}))

	result, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/noflow", Refresh: true})
	require.NoError(t, err)
	require.Nil(t, result.Traffic)
}

func TestServiceSubscriptionTrafficDoesNotPersistNodesReportsOrLastReport(t *testing.T) {
	sub := base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name:   "remote/provider",
		Type:   domain.SubscriptionTypeRemote,
		Format: "base64",
		Remote: &domain.RemoteInput{URL: subServer.URL},
		Meta:   map[string]string{"origin": "remote"},
	}))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name: "collections/live",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{
			Name: "provider",
			Type: "subscription",
			Ref:  domain.ResourceRef{Kind: "subscription", Name: "remote/provider"},
		}},
		Processors: []domain.ProcessorSpec{{
			Type:  "rename",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"mode":  "prefix",
				"value": "live-",
			}),
		}},
		Meta: map[string]string{"profile": "default"},
	}))

	result, err := svc.SubscriptionTraffic(context.Background(), domain.SubscriptionTrafficRequest{Name: "remote/provider", Refresh: true})
	require.NoError(t, err)
	require.NotNil(t, result.Traffic)

	stored, err := svc.GetSubscription(context.Background(), "remote/provider")
	require.NoError(t, err)
	require.Empty(t, stored.Nodes)
	require.Equal(t, "remote", stored.Meta["origin"])
}

func TestServiceSubscriptionTrafficCachesUntilForcedRefresh(t *testing.T) {
	ctx := context.Background()
	body := "ss://aes-128-gcm:secret@example.com:8388#first"
	upload := "1024"
	calls := 0
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Subscription-Userinfo", "upload="+upload+"; download=2048; total=10240")
		_, _ = w.Write([]byte(body))
	}))
	defer subServer.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutRuntimeSettings(ctx, domain.RuntimeSettings{
		CacheDefaults: domain.CacheDefaults{
			RemoteFetchTTLSeconds:         3600,
			SubscriptionTrafficTTLSeconds: 60,
		},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/live",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL},
	}))

	first, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/live"})
	require.NoError(t, err)
	require.NotNil(t, first.Traffic)
	require.Equal(t, int64(1024), first.Traffic.UploadBytes)

	body = "ss://aes-128-gcm:secret@example.com:8388#first\nss://aes-128-gcm:secret@example.org:8389#second"
	upload = "4096"
	cached, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/live"})
	require.NoError(t, err)
	require.Equal(t, int64(1024), cached.Traffic.UploadBytes)
	require.True(t, cached.Cached)
	require.Equal(t, 1, calls)

	fresh, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/live", Refresh: true})
	require.NoError(t, err)
	require.Equal(t, int64(4096), fresh.Traffic.UploadBytes)
	require.False(t, fresh.Cached)
	require.Equal(t, 2, calls)
}

func TestServiceSubscriptionTrafficRuntimeTTLExpiresCache(t *testing.T) {
	ctx := context.Background()
	upload := "1024"
	calls := 0
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Subscription-Userinfo", "upload="+upload+"; download=2048; total=10240")
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#first"))
	}))
	defer subServer.Close()

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	svc := service.New(service.WithFS(afero.NewMemMapFs()), service.WithClock(func() time.Time { return now }))
	require.NoError(t, svc.PutRuntimeSettings(ctx, domain.RuntimeSettings{
		CacheDefaults: domain.CacheDefaults{
			SubscriptionTrafficTTLSeconds: 1,
		},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "remote/live",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{URL: subServer.URL},
	}))

	first, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/live"})
	require.NoError(t, err)
	require.Equal(t, int64(1024), first.Traffic.UploadBytes)
	upload = "4096"
	now = now.Add(2 * time.Second)
	second, err := svc.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{Name: "remote/live"})
	require.NoError(t, err)
	require.Equal(t, int64(4096), second.Traffic.UploadBytes)
	require.False(t, second.Cached)
	require.Equal(t, 2, calls)
}
