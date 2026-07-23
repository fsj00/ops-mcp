package udp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/fsj00/ops-mcp/internal/connector/netutil"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Connector performs one-shot UDP datagram request/response exchanges.
type Connector struct {
	log *zap.Logger
}

func New(log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{log: log}
}

// Exchange sends one datagram to ip:port and waits for a single response packet.
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
	c.log.Debug("udp exchange",
		zap.String("addr", addr),
		zap.Int("request_bytes", len(payload)),
		zap.Int("max_response_bytes", maxResp),
		zap.Duration("timeout", timeout),
	)

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("udp resolve %s: %v", addr, err))
	}

	start := time.Now()
	var d net.Dialer
	d.Timeout = timeout
	conn, err := d.DialContext(ctx, "udp", raddr.String())
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("udp dial %s: %v", addr, err))
	}
	defer conn.Close() //nolint:errcheck

	deadlineAt := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadlineAt); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("udp set deadline: %v", err))
	}

	if _, err := conn.Write(payload); err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("udp write %s: %v", addr, err))
	}

	buf := make([]byte, maxResp)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("udp read %s: %v", addr, err))
	}
	resp := buf[:n]

	rtt := time.Since(start).Milliseconds()
	return &netutil.ExchangeResult{
		IP:            host,
		Port:          port,
		Protocol:      "udp",
		RequestBytes:  len(payload),
		ResponseBytes: len(resp),
		Hex:           netutil.BytesToHex(resp),
		Bytes:         netutil.BytesToInts(resp),
		RTTMs:         rtt,
	}, nil
}
