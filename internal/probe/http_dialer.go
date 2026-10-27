package probe

import (
	"context"
	"errors"
	"net"
)

// httpSafeDialer guards the core boundary because net/http dials and performs
// stream IO in its own goroutines, outside the request worker's panic guard.
type httpSafeDialer struct{ Dialer }

func (d httpSafeDialer) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	defer func() {
		if recover() != nil {
			conn = nil
			err = errors.New("node HTTP dial panicked")
		}
	}()
	conn, err = d.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("node HTTP dial returned no connection")
	}
	return &httpSafeConn{Conn: conn}, nil
}

type httpSafeConn struct{ net.Conn }

func (c *httpSafeConn) Read(p []byte) (n int, err error) {
	defer func() {
		if recover() != nil {
			n = 0
			err = errors.New("node HTTP read panicked")
		}
	}()
	return c.Conn.Read(p)
}
func (c *httpSafeConn) Write(p []byte) (n int, err error) {
	defer func() {
		if recover() != nil {
			n = 0
			err = errors.New("node HTTP write panicked")
		}
	}()
	return c.Conn.Write(p)
}
func (c *httpSafeConn) Close() (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("node HTTP close panicked")
		}
	}()
	return c.Conn.Close()
}
