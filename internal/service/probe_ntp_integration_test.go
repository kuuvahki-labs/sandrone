//go:build probe_singbox

package service_test

import (
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxUDPNTPRoundTripAndNodeIsolation(t *testing.T) {
	target, targetRequests := startProbeNTPResponder(t)
	working := startProbeSOCKSUDP(t, target, "forward")
	rejected := startProbeSOCKSUDP(t, target, "reject")
	dropped := startProbeSOCKSUDP(t, target, "drop")

	result, err := service.New().Probe(t.Context(), domain.ProbeRequest{
		Input: domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{
			working.node("working"), rejected.node("udp-rejected"), dropped.node("udp-dropped"),
		}},
		Method: domain.ProbeUDPNTP, Core: "sing-box",
		// The proxy maps this test-only destination to the loopback responder.
		// A direct connection cannot accidentally satisfy this test.
		NTPServer: "192.0.2.123", TimeoutMS: 1000, Attempts: 1, Concurrency: 3,
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 3)
	byName := make(map[string]domain.NodeProbeResult, len(result.Results))
	for _, item := range result.Results {
		byName[item.NodeName] = item
		require.Equal(t, "sing-box", item.Core)
		require.Equal(t, string(domain.ProbeUDPNTP), item.Method)
		require.Equal(t, "192.0.2.123:123", item.Target)
	}
	require.True(t, byName["working"].Alive, "%+v", byName["working"])
	require.Empty(t, byName["working"].ErrorCode)
	require.False(t, byName["udp-rejected"].Alive)
	require.Equal(t, string(domain.CodeProbeUDPNTPFailed), byName["udp-rejected"].ErrorCode)
	require.False(t, byName["udp-dropped"].Alive)
	require.Equal(t, string(domain.CodeProbeTimeout), byName["udp-dropped"].ErrorCode)
	require.Equal(t, 1, result.Report.Probe.SuccessCount)
	require.Equal(t, 2, result.Report.Probe.FailureCount)
	require.Zero(t, result.Report.Probe.UnsupportedCount)

	// Count both forwarding directions, not just the TCP SOCKS handshake.
	require.EqualValues(t, 1, working.requests.Load())
	require.Eventually(t, func() bool { return working.responses.Load() == 1 }, time.Second, time.Millisecond)
	require.EqualValues(t, 1, targetRequests.Load())
	require.EqualValues(t, 1, rejected.associations.Load())
	require.Zero(t, rejected.requests.Load())
	require.EqualValues(t, 1, dropped.requests.Load())
	require.Zero(t, dropped.responses.Load())
}

func startProbeNTPResponder(t *testing.T) (*net.UDPAddr, *atomic.Int32) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	var requests atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		packet := make([]byte, 512)
		for {
			n, peer, readErr := conn.ReadFromUDP(packet)
			if readErr != nil {
				return
			}
			if n != 48 || packet[0]&7 != 3 {
				t.Errorf("unexpected NTP request: %x", packet[:n])
				continue
			}
			requests.Add(1)
			response := make([]byte, 48)
			response[0], response[1] = 0x1c, 1 // NTP v3 server, stratum 1.
			response[2], response[3] = 4, 0xec
			copy(response[12:16], "LOCL")
			copy(response[24:32], packet[40:48])
			for _, offset := range []int{16, 32, 40} {
				binary.BigEndian.PutUint32(response[offset:], uint32(time.Now().Unix()+2208988800))
			}
			if _, writeErr := conn.WriteToUDP(response, peer); writeErr != nil {
				t.Errorf("write NTP response: %v", writeErr)
			}
		}
	}()
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})
	return conn.LocalAddr().(*net.UDPAddr), &requests
}

type probeSOCKSUDP struct {
	address      *net.TCPAddr
	associations atomic.Int32
	requests     atomic.Int32
	responses    atomic.Int32
}

func (p *probeSOCKSUDP) node(name string) domain.NodeIR {
	return domain.NodeIR{
		Name: name, Type: domain.NodeTypeSOCKS, Server: p.address.IP.String(), Port: uint16(p.address.Port),
	}
}

// startProbeSOCKSUDP implements only no-auth UDP ASSOCIATE and a single IPv4
// NTP destination. It never contacts the requested address or the public net.
func startProbeSOCKSUDP(t *testing.T, target *net.UDPAddr, mode string) *probeSOCKSUDP {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	relay, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	proxy := &probeSOCKSUDP{address: listener.Addr().(*net.TCPAddr)}
	accepted := make(chan net.Conn, 1)
	tcpDone, udpDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(tcpDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		accepted <- conn
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		header := make([]byte, 2)
		if _, readErr := io.ReadFull(conn, header); readErr != nil {
			return
		}
		if header[0] != 5 {
			t.Errorf("unexpected SOCKS version: %d", header[0])
			return
		}
		if _, readErr := io.CopyN(io.Discard, conn, int64(header[1])); readErr != nil {
			return
		}
		if _, writeErr := conn.Write([]byte{5, 0}); writeErr != nil {
			return
		}
		request := make([]byte, 4)
		if _, readErr := io.ReadFull(conn, request); readErr != nil {
			return
		}
		if request[0] != 5 || request[1] != 3 || request[2] != 0 {
			t.Errorf("expected UDP ASSOCIATE, got %x", request)
			return
		}
		addressLen := 0
		switch request[3] {
		case 1:
			addressLen = 4
		case 4:
			addressLen = 16
		default:
			t.Errorf("unexpected SOCKS association address type: %d", request[3])
			return
		}
		if _, readErr := io.CopyN(io.Discard, conn, int64(addressLen+2)); readErr != nil {
			return
		}
		proxy.associations.Add(1)
		reply := []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}
		binary.BigEndian.PutUint16(reply[8:], uint16(relay.LocalAddr().(*net.UDPAddr).Port))
		if mode == "reject" {
			reply[1] = 7 // Command not supported: UDP is unavailable.
		}
		if _, writeErr := conn.Write(reply); writeErr != nil || mode == "reject" {
			return
		}
		_, _ = io.Copy(io.Discard, conn) // Association lifetime follows TCP.
	}()
	go func() {
		defer close(udpDone)
		packet := make([]byte, 512)
		n, client, readErr := relay.ReadFromUDP(packet)
		if readErr != nil {
			return
		}
		if n != 58 || string(packet[:10]) != string([]byte{0, 0, 0, 1, 192, 0, 2, 123, 0, 123}) {
			t.Errorf("unexpected SOCKS UDP NTP datagram: %x", packet[:n])
			return
		}
		proxy.requests.Add(1)
		if mode == "drop" {
			return
		}
		upstream, dialErr := net.DialUDP("udp4", nil, target)
		if dialErr != nil {
			t.Errorf("open local NTP upstream: %v", dialErr)
			return
		}
		defer upstream.Close()
		_ = upstream.SetDeadline(time.Now().Add(2 * time.Second))
		if _, writeErr := upstream.Write(packet[10:n]); writeErr != nil {
			t.Errorf("forward NTP request: %v", writeErr)
			return
		}
		n, readErr = upstream.Read(packet[10:])
		if readErr != nil {
			t.Errorf("read NTP response: %v", readErr)
			return
		}
		if _, writeErr := relay.WriteToUDP(packet[:10+n], client); writeErr != nil {
			t.Errorf("forward NTP response: %v", writeErr)
			return
		}
		proxy.responses.Add(1)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		_ = relay.Close()
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
		<-tcpDone
		<-udpDone
	})
	return proxy
}
