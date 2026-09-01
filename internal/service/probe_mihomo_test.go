//go:build probe_mihomo

package service_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceMihomoURLTestWithLocalProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	svc := service.New()
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local-http-proxy",
				Type:   domain.NodeTypeHTTP,
				Server: host,
				Port:   port,
			}},
		},
		Method:         domain.ProbeURLTest,
		Core:           "mihomo",
		URL:            target.URL,
		ExpectedStatus: "200-299",
		TimeoutMS:      2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.Empty(t, result.Report.Warnings)
	require.Equal(t, "mihomo", result.Results[0].Core)
	require.Equal(t, "mihomo_url_test", result.Report.Probe.Backend)
}

func TestServiceMihomoURLTestIsolatesUnsupportedTLSClientFingerprint(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	result, err := service.New().Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
			{Name: "valid", Type: domain.NodeTypeHTTP, Server: host, Port: port},
			{
				Name: "unsupported", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
				UUID: "11111111-1111-1111-1111-111111111111",
				TLS:  &domain.TLSOptions{Enabled: true, ClientFingerprint: "unsafe"},
			},
		}},
		Method: domain.ProbeURLTest, Core: "mihomo", URL: target.URL,
		ExpectedStatus: "200-299", TimeoutMS: 2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.NotEqual(t, string(domain.CodeProbeCoreStartFailed), result.Results[0].ErrorCode)
	require.True(t, containsWarning(result.Report.Warnings, "node_validation_dropped", "tls.client_fingerprint"))
}

func TestServiceMihomoURLTestIsolatesInvalidRealityPublicKey(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	result, err := service.New().Probe(t.Context(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
			{Name: "valid", Type: domain.NodeTypeHTTP, Server: host, Port: port},
			{
				Name: "invalid-reality", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
				UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
				TLS: &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{Enabled: true}},
			},
		}},
		Method: domain.ProbeURLTest, Core: "mihomo", URL: target.URL,
		ExpectedStatus: "200-299", TimeoutMS: 2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.NotEqual(t, string(domain.CodeProbeCoreStartFailed), result.Results[0].ErrorCode)
	require.True(t, containsWarning(result.Report.Warnings, "node_validation_dropped", "tls.reality.public_key"))
}

func TestServiceMihomoURLTestIsolatesCustomECHDNSTransport(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	result, err := service.New().Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
			{Name: "valid", Type: domain.NodeTypeHTTP, Server: host, Port: port},
			{
				Name: "ech", Type: domain.NodeTypeVLESS, Server: "ech.example", Port: 443,
				UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
				TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{
					Enabled: true, QueryServerName: "ip.gs", DNS: "udp://8.8.8.8",
				}},
			},
		}},
		Method: domain.ProbeURLTest, Core: "mihomo", URL: target.URL,
		ExpectedStatus: "200-299", TimeoutMS: 2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 2)
	require.True(t, result.Results[0].Alive)
	require.False(t, result.Results[1].Alive)
	require.Equal(t, string(domain.CodeProbeNodeUnsupported), result.Results[1].ErrorCode)
	require.Equal(t, 1, result.Report.Probe.SuccessCount)
	require.Equal(t, 1, result.Report.Probe.UnsupportedCount)
	require.Zero(t, result.Report.Probe.FailureCount)
	require.True(t, containsWarning(result.Report.Warnings, "render_node_skipped", "vless"))
}

func TestServiceMihomoURLTestVMessDefaultsEmptyCipherBeforeCoreParse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	result, err := service.New().Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name: "vmess-default", Type: domain.NodeTypeVMess, Server: "127.0.0.1", Port: 1,
				UUID: "11111111-1111-1111-1111-111111111111",
			}},
		},
		Method:         domain.ProbeURLTest,
		Core:           "mihomo",
		URL:            target.URL,
		ExpectedStatus: "200-299",
		TimeoutMS:      250,
		Attempts:       1,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].Alive)
	require.NotEqual(t, string(domain.CodeProbeInvalidTarget), result.Results[0].ErrorCode)
	require.NotContains(t, result.Results[0].Error, "unset fields: cipher")
}

func TestServiceMihomoURLTestRejectsUnexpectedStatus(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	svc := service.New()
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local-http-proxy",
				Type:   domain.NodeTypeHTTP,
				Server: host,
				Port:   port,
			}},
		},
		Method:         domain.ProbeURLTest,
		Core:           "mihomo",
		URL:            target.URL,
		ExpectedStatus: "200/201",
		TimeoutMS:      2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].Alive)
	require.Equal(t, "probe_core_api_failed", result.Results[0].ErrorCode)
	require.Contains(t, result.Results[0].Error, "204")
	require.Contains(t, result.Results[0].Error, "200/201")
}

func TestServiceMihomoURLTestRejectsInvalidExpectedStatus(t *testing.T) {
	targetHit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHit <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	proxyAddr, closeProxy := startConnectProxy(t)
	defer closeProxy()
	host, port := splitHostPort(t, proxyAddr)

	svc := service.New()
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local-http-proxy",
				Type:   domain.NodeTypeHTTP,
				Server: host,
				Port:   port,
			}},
		},
		Method:         domain.ProbeURLTest,
		Core:           "mihomo",
		URL:            target.URL,
		ExpectedStatus: "not-a-status",
		TimeoutMS:      2000,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget))
	select {
	case <-targetHit:
		t.Fatal("mihomo requested the target before rejecting expected_status")
	default:
	}
}

func startConnectProxy(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConnectProxyConn(conn)
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func handleConnectProxyConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = fmt.Fprint(conn, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}
	target, err := net.Dial("tcp", req.Host)
	if err != nil {
		_, _ = fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer target.Close()
	_, _ = fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(target, reader)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(conn, target)
		errc <- err
	}()
	<-errc
}

func splitHostPort(t *testing.T, addr string) (string, uint16) {
	t.Helper()
	host, portValue, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.ParseUint(portValue, 10, 16)
	require.NoError(t, err)
	return host, uint16(port)
}
