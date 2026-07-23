package udp

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/connector/netutil"
)

func TestExchangeEcho(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		_, _ = pc.WriteTo(buf[:n], addr)
	}()

	_, portStr, err := net.SplitHostPort(pc.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	c := New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := c.Exchange(ctx, netutil.ExchangeRequest{
		IP:      "127.0.0.1",
		Port:    port,
		Data:    []interface{}{1.0, 2.0, 3.0, 10.0},
		Timeout: "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Protocol != "udp" || res.Hex != "0102030a" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestExchangeTimeout(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	// no responder

	_, portStr, _ := net.SplitHostPort(pc.LocalAddr().String())
	port, _ := strconv.Atoi(portStr)

	c := New(nil)
	_, err = c.Exchange(context.Background(), netutil.ExchangeRequest{
		IP:      "127.0.0.1",
		Port:    port,
		Data:    "01",
		Timeout: "100ms",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// TestExchangeDockerEcho hits make net-up (127.0.0.1:19091); skips if unreachable.
func TestExchangeDockerEcho(t *testing.T) {
	// Probe TCP health port of the same compose service first.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:19090", 200*time.Millisecond)
	if err != nil {
		t.Skip("docker net-echo not running (make net-up):", err)
	}
	_ = conn.Close()

	c := New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := c.Exchange(ctx, netutil.ExchangeRequest{
		IP: "127.0.0.1", Port: 19091, Data: "0102030a", Timeout: "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Hex != "0102030a" {
		t.Fatalf("hex=%s", res.Hex)
	}
}
