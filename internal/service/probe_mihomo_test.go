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
		Method:    domain.ProbeURLTest,
		Core:      "mihomo",
		URL:       target.URL,
		TimeoutMS: 2000,
	})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.Equal(t, "mihomo", result.Results[0].Core)
	require.Equal(t, "mihomo_url_test", result.Report.Probe.Backend)
}

func TestServiceMihomoURLTestRejectsInvalidExpectedStatus(t *testing.T) {
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
		URL:            "http://127.0.0.1/generate_204",
		ExpectedStatus: "not-a-status",
		TimeoutMS:      2000,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget))
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
