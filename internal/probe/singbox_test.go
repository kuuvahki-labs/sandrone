//go:build probe_singbox

package probe

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

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
