package service_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceConvertLogsSummaryWithoutInputContent(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := service.New(service.WithLogger(logger))

	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "json-nodes",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Body)
	out := logs.String()
	require.Contains(t, out, `"msg":"service convert completed"`)
	require.Contains(t, out, `"operation":"convert"`)
	require.Contains(t, out, `"from_format":"uri-list"`)
	require.Contains(t, out, `"to_format":"json-nodes"`)
	require.Contains(t, out, `"node_count":1`)
	require.NotContains(t, out, "secret@example.com")
	require.NotContains(t, out, "aes-128-gcm")
}

func TestServiceProbeLogsSummary(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := service.New(
		service.WithLogger(logger),
		service.WithProbeEngine(fakeProbeEngine{probe: func(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
			return &domain.ProbeResult{
				Results: []domain.NodeProbeResult{{
					NodeName:   "n",
					Method:     string(req.Method),
					Alive:      true,
					DurationMS: 4,
					CheckedAt:  time.Date(2026, 6, 24, 1, 2, 3, 0, time.UTC),
				}},
				Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake", Method: string(req.Method), SuccessCount: 1}},
			}, nil
		}}),
	)

	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "n",
				Server: "127.0.0.1",
				Port:   443,
			}},
		},
		Method: domain.ProbeTCPConnect,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	out := logs.String()
	require.Contains(t, out, `"msg":"service probe completed"`)
	require.Contains(t, out, `"operation":"probe"`)
	require.Contains(t, out, `"method":"tcp_connect"`)
	require.Contains(t, out, `"node_count":1`)
	require.Contains(t, out, `"success_count":1`)
}

func TestServicePutFileLogsResourceWriteWithoutContent(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := service.New(service.WithFS(afero.NewMemMapFs()), service.WithLogger(logger))

	err := svc.PutFile(context.Background(), domain.FileSpec{
		Name:   "client.yaml",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "secret-body"},
	})

	require.NoError(t, err)
	out := logs.String()
	require.Contains(t, out, `"msg":"service resource updated"`)
	require.Contains(t, out, `"operation":"put_resource"`)
	require.Contains(t, out, `"resource_type":"file"`)
	require.Contains(t, out, `"resource_name":"client.yaml"`)
	require.NotContains(t, out, "secret-body")
}
