package probe

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type panicHTTPDialer struct{}

func (panicHTTPDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	panic("secret-dial-detail")
}

type panicHTTPConn struct{ net.Conn }

func (*panicHTTPConn) Read([]byte) (int, error)  { panic("secret-read-detail") }
func (*panicHTTPConn) Write([]byte) (int, error) { panic("secret-write-detail") }
func (*panicHTTPConn) Close() error              { panic("secret-close-detail") }

func TestNodeHTTPGuardsNetHTTPGoroutineCoreCalls(t *testing.T) {
	_, err := (httpSafeDialer{Dialer: panicHTTPDialer{}}).DialContext(context.Background(), "tcp", "example.com:80")
	require.EqualError(t, err, "node HTTP dial panicked")
	conn := &httpSafeConn{Conn: &panicHTTPConn{}}
	_, err = conn.Read(nil)
	require.EqualError(t, err, "node HTTP read panicked")
	_, err = conn.Write(nil)
	require.EqualError(t, err, "node HTTP write panicked")
	require.EqualError(t, conn.Close(), "node HTTP close panicked")
}
