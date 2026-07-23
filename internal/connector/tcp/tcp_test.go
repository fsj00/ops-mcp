package tcp

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/connector/netutil"
)

func TestExchangeEcho(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf, err := io.ReadAll(conn)
		if err != nil || len(buf) == 0 {
			return
		}
		_, _ = conn.Write(buf)
	}()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
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
		Data:    "0102030a",
		Timeout: "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Protocol != "tcp" || res.Hex != "0102030a" || res.ResponseBytes != 4 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.Bytes) != 4 || res.Bytes[3] != 10 {
		t.Fatalf("bytes=%v", res.Bytes)
	}
}

func TestExchangeInvalidParams(t *testing.T) {
	c := New(nil)
	_, err := c.Exchange(context.Background(), netutil.ExchangeRequest{IP: "", Port: 1, Data: "01"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestExchangeDockerEcho hits make net-up (127.0.0.1:19090); skips if unreachable.
func TestExchangeDockerEcho(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:19090", 200*time.Millisecond)
	if err != nil {
		t.Skip("docker net-echo not running (make net-up):", err)
	}
	_ = conn.Close()

	c := New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := c.Exchange(ctx, netutil.ExchangeRequest{
		IP: "127.0.0.1", Port: 19090, Data: "0102030a", Timeout: "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Hex != "0102030a" {
		t.Fatalf("hex=%s", res.Hex)
	}
}
