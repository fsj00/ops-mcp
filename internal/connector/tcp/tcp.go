package tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/fsj00/ops-mcp/internal/connector/netutil"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Connector performs one-shot TCP request/response exchanges.
type Connector struct {
	log *zap.Logger
}

func New(log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{log: log}
}

// Exchange dials ip:port, writes data, reads until EOF / deadline / max bytes, then closes.
func (c *Connector) Exchange(ctx context.Context, req netutil.ExchangeRequest) (*netutil.ExchangeResult, error) {
	host, port, err := netutil.ValidateTarget(req.IP, req.Port)
	if err != nil {
		return nil, err
	}
	payload, err := netutil.DecodeData(req.Data)
	if err != nil {
		return nil, err
	}
	maxResp, err := netutil.ResolveMaxResponseBytes(req.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	var deadline time.Time
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	timeout, err := netutil.ResolveTimeout(deadline, req.Timeout, netutil.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	addr := netutil.JoinHostPort(host, port)
	c.log.Debug("tcp exchange",
		zap.String("addr", addr),
		zap.Int("request_bytes", len(payload)),
		zap.Int("max_response_bytes", maxResp),
		zap.Duration("timeout", timeout),
	)

	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("tcp dial %s: %v", addr, err))
	}
	defer conn.Close() //nolint:errcheck

	deadlineAt := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadlineAt); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("tcp set deadline: %v", err))
	}

	if _, err := conn.Write(payload); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("tcp write %s: %v", addr, err))
	}
	// Half-close write so peers that read until EOF can reply (request/response).
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	buf := make([]byte, 0, min(maxResp, 4096))
	tmp := make([]byte, min(maxResp, 8192))
	for len(buf) < maxResp {
		n, rerr := conn.Read(tmp[:min(len(tmp), maxResp-len(buf))])
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			var ne net.Error
			if errors.As(rerr, &ne) && ne.Timeout() && len(buf) > 0 {
				// Peer kept connection open; return bytes received before deadline.
				break
			}
			if len(buf) == 0 {
				return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("tcp read %s: %v", addr, rerr))
			}
			break
		}
	}

	rtt := time.Since(start).Milliseconds()
	return &netutil.ExchangeResult{
		IP:            host,
		Port:          port,
		Protocol:      "tcp",
		RequestBytes:  len(payload),
		ResponseBytes: len(buf),
		Hex:           netutil.BytesToHex(buf),
		Bytes:         netutil.BytesToInts(buf),
		RTTMs:         rtt,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
