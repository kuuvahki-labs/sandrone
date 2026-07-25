package probe_test

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

func TestTCPConnectSuccessWithLocalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	engine := probe.New()
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeTCPConnect,
		TimeoutMS: 1000,
	}, []domain.NodeIR{{
		ID:     "node-1",
		Name:   "local",
		Server: "127.0.0.1",
		Port:   uint16(addr.Port),
	}})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.Equal(t, string(domain.ProbeTCPConnect), result.Results[0].Method)
	require.Equal(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(addr.Port)), result.Results[0].Target)
	require.GreaterOrEqual(t, result.Results[0].DurationMS, 1)
	require.Equal(t, 1, result.Report.Probe.SuccessCount)
	require.Equal(t, 0, result.Report.Probe.FailureCount)
	_ = ln.Close()
	<-done
}

func TestExplicitUDPNTPSelectsCoreBackend(t *testing.T) {
	engine := probe.New(probe.WithBackend(fakeCoreBackend{method: domain.ProbeUDPNTP, core: "sing-box"}))
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeUDPNTP,
		NTPServer: "time.example.com",
	}, []domain.NodeIR{{
		Name:   "hy2",
		Type:   domain.NodeTypeHysteria2,
		Server: "example.com",
		Port:   443,
	}})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, string(domain.ProbeUDPNTP), result.Results[0].Method)
	require.Equal(t, "time.example.com:123", result.Results[0].Target)
	require.Equal(t, "sing-box", result.Results[0].Core)
	require.Equal(t, string(domain.ProbeUDPNTP), result.Report.Probe.Method)
}

func TestExplicitURLTestSelectsRequestedCore(t *testing.T) {
	engine := probe.New(probe.WithBackend(fakeCoreBackend{method: domain.ProbeURLTest, core: "mihomo"}))
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method: domain.ProbeURLTest,
		Core:   "mihomo",
		URL:    "https://example.com/generate_204",
	}, []domain.NodeIR{{Name: "proxy", Type: domain.NodeTypeHTTP}})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, string(domain.ProbeURLTest), result.Results[0].Method)
	require.Equal(t, "https://example.com/generate_204", result.Results[0].Target)
	require.Equal(t, "mihomo", result.Results[0].Core)
}

func TestTCPConnectDialFailure(t *testing.T) {
	engine := probe.New(probe.WithDialer(failingDialer{}))
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeTCPConnect,
		TimeoutMS: 100,
	}, []domain.NodeIR{{
		ID:           "node-failed",
		Name:         "failed",
		Type:         domain.NodeTypeVLESS,
		Server:       "127.0.0.1",
		Port:         443,
		SourceFormat: "uri-list",
	}})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].Alive)
	require.Equal(t, "probe_tcp_failed", result.Results[0].ErrorCode)
	require.Equal(t, 0, result.Report.Probe.SuccessCount)
	require.Equal(t, 1, result.Report.Probe.FailureCount)
	require.Len(t, result.Report.Warnings, 1)
	require.Equal(t, "probe_tcp_failed", result.Report.Warnings[0].Code)
	require.Equal(t, "failed", result.Report.Warnings[0].Node)
	require.NotNil(t, result.Report.Warnings[0].NodeIndex)
	require.Equal(t, 0, *result.Report.Warnings[0].NodeIndex)
	require.NotNil(t, result.Report.Warnings[0].NodeContext)
	require.Equal(t, "uri-list", result.Report.Warnings[0].NodeContext.Format)
	require.Equal(t, "failed", result.Report.Warnings[0].NodeContext.Name)
	require.Equal(t, domain.NodeTypeVLESS, result.Report.Warnings[0].NodeContext.Type)
	require.Equal(t, "127.0.0.1", result.Report.Warnings[0].NodeContext.Server)
	require.Equal(t, uint16(443), result.Report.Warnings[0].NodeContext.Port)
}

func TestTCPConnectRetriesAttempts(t *testing.T) {
	dialer := &countingDialer{failures: 1}
	engine := probe.New(probe.WithDialer(dialer))
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:   domain.ProbeTCPConnect,
		Attempts: 2,
	}, []domain.NodeIR{{Name: "retry", Server: "example.com", Port: 443}})

	require.NoError(t, err)
	require.True(t, result.Results[0].Alive)
	require.Equal(t, int32(2), atomic.LoadInt32(&dialer.calls))
}

func TestTCPConnectTimeout(t *testing.T) {
	engine := probe.New(probe.WithDialer(blockingDialer{}))
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeTCPConnect,
		TimeoutMS: 10,
	}, []domain.NodeIR{{Name: "timeout", Server: "example.com", Port: 443}})

	require.NoError(t, err)
	require.False(t, result.Results[0].Alive)
	require.Equal(t, "probe_timeout", result.Results[0].ErrorCode)
}

func TestTCPConnectConcurrencyLimit(t *testing.T) {
	dialer := &trackingDialer{delay: 10 * time.Millisecond}
	engine := probe.New(probe.WithDialer(dialer))
	nodes := make([]domain.NodeIR, 8)
	for i := range nodes {
		nodes[i] = domain.NodeIR{
			Name:   "node-" + strconv.Itoa(i),
			Server: "example.com",
			Port:   443,
		}
	}
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:      domain.ProbeTCPConnect,
		Concurrency: 2,
	}, nodes)

	require.NoError(t, err)
	require.Len(t, result.Results, len(nodes))
	require.LessOrEqual(t, atomic.LoadInt64(&dialer.max), int64(2))
}

func TestCoreURLTestRoutesByCore(t *testing.T) {
	engine := probe.New(
		probe.WithBackend(fakeCoreBackend{method: domain.ProbeURLTest, core: "mihomo"}),
		probe.WithBackend(fakeCoreBackend{method: domain.ProbeURLTest, core: "sing-box"}),
	)

	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method: domain.ProbeURLTest,
		Core:   "sing-box",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "fake-sing-box", result.Report.Probe.Backend)
	require.Equal(t, "sing-box", result.Report.Probe.Core)
}

func TestCoreURLTestRequiresCoreWhenAmbiguous(t *testing.T) {
	engine := probe.New(
		probe.WithBackend(fakeCoreBackend{method: domain.ProbeURLTest, core: "mihomo"}),
		probe.WithBackend(fakeCoreBackend{method: domain.ProbeURLTest, core: "sing-box"}),
	)

	_, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method: domain.ProbeURLTest,
	}, nil)

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestUnsupportedProbeMethodIsInvalidArgument(t *testing.T) {
	engine := probe.New()

	_, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method: domain.ProbeMethod("future"),
	}, nil)

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

type fakeCoreBackend struct {
	method domain.ProbeMethod
	core   string
}

func (b fakeCoreBackend) Method() domain.ProbeMethod { return b.method }

func (b fakeCoreBackend) Core() string { return b.core }

func (b fakeCoreBackend) Name() string { return "fake-" + b.core }

func (b fakeCoreBackend) Version() string { return "" }

func (b fakeCoreBackend) Probe(_ context.Context, req probe.BackendRequest, _ []domain.NodeIR) (*domain.ProbeResult, error) {
	target := req.Probe.URL
	if req.Probe.Method == domain.ProbeUDPNTP {
		target = req.Probe.NTPServer + ":123"
	}
	report := domain.Report{Probe: &domain.ProbeReport{
		Backend:      b.Name(),
		Method:       string(req.Probe.Method),
		Core:         req.Probe.Core,
		SuccessCount: 1,
	}}
	return &domain.ProbeResult{
		Results: []domain.NodeProbeResult{{
			NodeName:   "n",
			Method:     string(req.Probe.Method),
			Target:     target,
			Core:       req.Probe.Core,
			Alive:      true,
			DurationMS: 7,
			CheckedAt:  time.Now(),
		}},
		Report: report,
	}, nil
}

type countingDialer struct {
	calls    int32
	failures int32
}

func (d *countingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	call := atomic.AddInt32(&d.calls, 1)
	if call <= d.failures {
		return nil, errors.New("temporary failure")
	}
	return pipeConn(), nil
}

type blockingDialer struct{}

func (blockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type failingDialer struct{}

func (failingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("connection refused")
}

type trackingDialer struct {
	current int64
	max     int64
	delay   time.Duration
}

func (d *trackingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	current := atomic.AddInt64(&d.current, 1)
	for {
		max := atomic.LoadInt64(&d.max)
		if current <= max || atomic.CompareAndSwapInt64(&d.max, max, current) {
			break
		}
	}
	defer atomic.AddInt64(&d.current, -1)
	select {
	case <-time.After(d.delay):
		return pipeConn(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func pipeConn() net.Conn {
	left, right := net.Pipe()
	_ = right.Close()
	return left
}
