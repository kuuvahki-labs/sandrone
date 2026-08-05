//go:build probe_singbox

package probe

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSingBoxURLTestWorkerPanicBecomesSanitizedFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	listenerDone := make(chan struct{})
	defer close(listenerDone)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		<-listenerDone
	}()
	host, portString, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portString)
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"outbounds": []map[string]any{{
			"type":        "vless",
			"tag":         "panic-vless",
			"server":      host,
			"server_port": port,
			"uuid":        "11111111-1111-1111-1111-111111111111",
			"tls": map[string]any{
				"enabled":  false,
				"insecure": true,
			},
		}},
	})
	require.NoError(t, err)

	engine := New()
	result, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeURLTest,
		Core:      "sing-box",
		URL:       "http://127.0.0.1:1",
		TimeoutMS: 1000,
	}, []domain.NodeIR{{
		Name:   "panic-vless",
		Type:   domain.NodeTypeVLESS,
		Server: host,
		Port:   uint16(port),
	}}, Payload{Core: "sing-box", Format: "sing-box", Body: payload})

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.False(t, result.Results[0].Alive)
	require.Equal(t, string(domain.CodeProbeCoreAPIFailed), result.Results[0].ErrorCode)
	require.Equal(t, "probe worker panicked", result.Results[0].Error)
	require.Equal(t, "singbox_url_test", result.Results[0].Backend)
	require.Equal(t, 0, result.Report.Probe.SuccessCount)
	require.Equal(t, 1, result.Report.Probe.FailureCount)
	require.Equal(t, map[string]int{string(domain.CodeProbeCoreAPIFailed): 1}, result.Report.Probe.ErrorCounts)
	require.Len(t, result.Report.Warnings, 1)
	require.Equal(t, string(domain.CodeProbeCoreAPIFailed), result.Report.Warnings[0].Code)
	require.Equal(t, "probe_core_api_failed: probe worker panicked", result.Report.Warnings[0].Message)
}

func TestSingBoxNTPRoundTripUsesAttemptDeadline(t *testing.T) {
	attemptDeadline := time.Now().Add(25 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), attemptDeadline)
	defer cancel()
	conn := &deadlineRecordingPacketConn{}
	outbound := fakePacketOutbound{conn: conn}
	backend := &SingBoxNTPBackend{now: func() time.Time {
		return attemptDeadline.Add(-time.Millisecond)
	}}

	_, err := backend.ntpRoundTrip(ctx, outbound, metadata.Socksaddr{})

	require.Error(t, err)
	require.False(t, conn.deadline.IsZero())
	require.WithinDuration(t, attemptDeadline, conn.deadline, time.Millisecond)
}

type fakePacketOutbound struct {
	conn net.PacketConn
}

func (o fakePacketOutbound) ListenPacket(context.Context, metadata.Socksaddr) (net.PacketConn, error) {
	return o.conn, nil
}

type deadlineRecordingPacketConn struct {
	deadline time.Time
}

func (c *deadlineRecordingPacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, context.DeadlineExceeded
}

func (c *deadlineRecordingPacketConn) WriteTo([]byte, net.Addr) (int, error) {
	return 48, nil
}

func (c *deadlineRecordingPacketConn) Close() error { return nil }

func (c *deadlineRecordingPacketConn) LocalAddr() net.Addr { return nil }

func (c *deadlineRecordingPacketConn) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *deadlineRecordingPacketConn) SetReadDeadline(time.Time) error { return nil }

func (c *deadlineRecordingPacketConn) SetWriteDeadline(time.Time) error { return nil }
