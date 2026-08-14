//go:build probe_singbox

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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxURLTestWithLocalProxy(t *testing.T) {
	targetHit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		select {
		case targetHit <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	proxyAddr, closeProxy := startSingBoxConnectProxy(t, target.Listener.Addr().String())
	defer closeProxy()
	host, port := splitSingBoxHostPort(t, proxyAddr)

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
		Core:           "sing-box",
		URL:            target.URL,
		ExpectedStatus: "200-299",
		TimeoutMS:      2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.Empty(t, result.Report.Warnings)
	require.Equal(t, "sing-box", result.Results[0].Core)
	require.Equal(t, "singbox_url_test", result.Report.Probe.Backend)
	select {
	case <-targetHit:
	case <-time.After(time.Second):
		t.Fatal("sing-box did not request the configured HTTP test target")
	}
}

func TestServiceSingBoxURLTestAcceptsDisabledVLESSPacketEncoding(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		_ = conn.Close()
	}()
	host, port := splitSingBoxHostPort(t, listener.Addr().String())

	svc := service.New()
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name: "vless-none", Type: domain.NodeTypeVLESS, Server: host, Port: port,
				UUID: "11111111-1111-1111-1111-111111111111", PacketEncoding: "none",
			}},
		},
		Method:    domain.ProbeURLTest,
		Core:      "sing-box",
		URL:       "http://127.0.0.1:1",
		TimeoutMS: 1000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].Alive)
	require.NotEqual(t, string(domain.CodeProbeCoreStartFailed), result.Results[0].ErrorCode)
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("sing-box did not initialize the VLESS outbound")
	}
}

func TestServiceSingBoxURLTestRejectsUnexpectedStatus(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	proxyAddr, closeProxy := startSingBoxConnectProxy(t, target.Listener.Addr().String())
	defer closeProxy()
	host, port := splitSingBoxHostPort(t, proxyAddr)

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
		Core:           "sing-box",
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

func TestServiceSingBoxProbeIsolatesInvalidHysteriaBandwidth(t *testing.T) {
	targetHit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		select {
		case targetHit <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	proxyAddr, closeProxy := startSingBoxConnectProxy(t, target.Listener.Addr().String())
	defer closeProxy()
	proxyHost, proxyPort := splitSingBoxHostPort(t, proxyAddr)

	svc := service.New()
	result, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{
				{
					Name: "invalid-hysteria", Type: domain.NodeTypeHysteria,
					Server: "invalid.example", Port: 443,
					TLS:      &domain.TLSOptions{Enabled: true},
					Hysteria: &domain.HysteriaOptions{Up: "fast", DownMbps: 100},
				},
				{
					Name: "valid-http", Type: domain.NodeTypeHTTP,
					Server: proxyHost, Port: proxyPort,
				},
			},
		},
		Method:         domain.ProbeURLTest,
		Core:           "sing-box",
		URL:            target.URL,
		ExpectedStatus: "200-299",
		TimeoutMS:      2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 2)
	require.Equal(t, "invalid-hysteria", result.Results[0].NodeName)
	require.False(t, result.Results[0].Alive)
	require.Equal(t, string(domain.CodeProbeInvalidTarget), result.Results[0].ErrorCode)
	require.Equal(t, "valid-http", result.Results[1].NodeName)
	require.True(t, result.Results[1].Alive)
	warningCodes := make([]string, 0, len(result.Report.Warnings))
	for _, warning := range result.Report.Warnings {
		warningCodes = append(warningCodes, warning.Code)
	}
	require.Contains(t, warningCodes, "parse_unknown_field")
	require.Contains(t, warningCodes, "render_node_skipped")
	select {
	case <-targetHit:
	case <-time.After(time.Second):
		t.Fatal("sing-box did not probe the valid HTTP node")
	}
}

func TestServiceSingBoxURLTestRejectsInvalidExpectedStatus(t *testing.T) {
	targetHit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHit <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	host, port := splitSingBoxHostPort(t, target.Listener.Addr().String())

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
		Core:           "sing-box",
		URL:            target.URL,
		ExpectedStatus: "not-a-status",
		TimeoutMS:      2000,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget))
	select {
	case <-targetHit:
		t.Fatal("sing-box requested the target before rejecting expected_status")
	default:
	}
}

func TestServiceSingBoxURLTestRejectsInvalidURL(t *testing.T) {
	svc := service.New()
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local-http-proxy",
				Type:   domain.NodeTypeHTTP,
				Server: "127.0.0.1",
				Port:   8080,
			}},
		},
		Method:    domain.ProbeURLTest,
		Core:      "sing-box",
		URL:       "ftp://127.0.0.1/generate_204",
		TimeoutMS: 2000,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget))
}

func startSingBoxConnectProxy(t *testing.T, allowedTarget string) (string, func()) {
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
			go handleSingBoxConnectProxyConn(conn, allowedTarget)
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func handleSingBoxConnectProxyConn(conn net.Conn, allowedTarget string) {
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
	if req.Host != allowedTarget {
		_, _ = fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
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

func splitSingBoxHostPort(t *testing.T, addr string) (string, uint16) {
	t.Helper()
	host, portValue, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.ParseUint(portValue, 10, 16)
	require.NoError(t, err)
	return host, uint16(port)
}
