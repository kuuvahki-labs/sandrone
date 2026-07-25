package probe

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type TCPBackend struct {
	dialer Dialer
	now    func() time.Time
}

func NewTCPBackend(dialer Dialer, now func() time.Time) *TCPBackend {
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	if now == nil {
		now = time.Now
	}
	return &TCPBackend{dialer: dialer, now: now}
}

func (b *TCPBackend) Method() domain.ProbeMethod { return domain.ProbeTCPConnect }

func (b *TCPBackend) Name() string { return "tcp_connect" }

func (b *TCPBackend) Version() string { return "" }

func (b *TCPBackend) Probe(ctx context.Context, backendReq BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error) {
	req := backendReq.Probe
	timeout := timeoutFromRequest(req)
	attempts := attemptsFromRequest(req)
	concurrency := concurrencyFromRequest(req)

	results := make([]domain.NodeProbeResult, len(nodes))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = resultForError(req, node, "probe_context_canceled", ctx.Err(), b.now())
				return
			}
			results[i] = b.probeNode(ctx, req, node, timeout, attempts)
		}()
	}
	wg.Wait()

	report := reportForResults(b.Name(), b.Version(), string(req.Method), req.Core, nodes, results)
	for i := range results {
		results[i].Backend = b.Name()
	}
	if req.ExpectedStatus != "" {
		report.Warnings = append(report.Warnings, domain.Warning{
			Code:    "probe_expected_status_unsupported",
			Message: "expected_status is only meaningful for url_test; tcp_connect ignored it",
		})
	}
	return &domain.ProbeResult{Results: results, Report: report}, nil
}

func (b *TCPBackend) probeNode(ctx context.Context, req domain.ProbeRequest, node domain.NodeIR, timeout time.Duration, attempts int) domain.NodeProbeResult {
	if node.Server == "" {
		return resultForError(req, node, "probe_node_invalid", errors.New("node server is required"), b.now())
	}
	if node.Port == 0 {
		return resultForError(req, node, "probe_node_invalid", errors.New("node port is required"), b.now())
	}
	address := net.JoinHostPort(node.Server, strconv.Itoa(int(node.Port)))
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		start := b.now()
		conn, err := b.dialer.DialContext(attemptCtx, "tcp", address)
		elapsed := b.now().Sub(start)
		cancel()
		if err == nil {
			_ = conn.Close()
			return successResult(req, node, int(elapsed/time.Millisecond), b.now())
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return resultForError(req, node, errorCodeForDial(lastErr), lastErr, b.now())
}

func errorCodeForDial(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "probe_timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "probe_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "probe_context_canceled"
	}
	return "probe_tcp_failed"
}
