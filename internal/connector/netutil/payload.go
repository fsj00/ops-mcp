package netutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fsj00/ops-mcp/internal/model"
)

const (
	// DefaultTimeout is used when request timeout is omitted.
	DefaultTimeout = 5 * time.Second
	// DefaultMaxResponseBytes caps a single exchange response.
	DefaultMaxResponseBytes = 65536
	// AbsoluteMaxResponseBytes is a hard upper bound regardless of request.
	AbsoluteMaxResponseBytes = 1 << 20 // 1 MiB
)

// ExchangeRequest is the JS-facing tcp/udp.exchange input.
type ExchangeRequest struct {
	IP               string      `json:"ip"`
	Port             int         `json:"port"`
	Data             interface{} `json:"data"`
	Timeout          string      `json:"timeout"`
	MaxResponseBytes int         `json:"max_response_bytes"`
}

// ExchangeResult is the JS-facing tcp/udp.exchange result.
type ExchangeResult struct {
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Protocol      string `json:"protocol"`
	RequestBytes  int    `json:"request_bytes"`
	ResponseBytes int    `json:"response_bytes"`
	Hex           string `json:"hex"`
	Bytes         []int  `json:"bytes"`
	RTTMs         int64  `json:"rtt_ms"`
}

// DecodeData accepts a hex string or an array of 0–255 integers.
func DecodeData(data interface{}) ([]byte, error) {
	if data == nil {
		return nil, model.NewAppError(model.ErrInvalidParams, "data is required")
	}
	switch v := data.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, model.NewAppError(model.ErrInvalidParams, "data is required")
		}
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, ":", "")
		s = strings.ReplaceAll(s, "-", "")
		if len(s)%2 != 0 {
			return nil, model.NewAppError(model.ErrInvalidParams, "data hex length must be even")
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("data hex: %v", err))
		}
		if len(b) == 0 {
			return nil, model.NewAppError(model.ErrInvalidParams, "data is empty")
		}
		return b, nil
	case []byte:
		if len(v) == 0 {
			return nil, model.NewAppError(model.ErrInvalidParams, "data is empty")
		}
		return append([]byte(nil), v...), nil
	case []interface{}:
		return intsFromAny(v)
	case []int:
		out := make([]byte, len(v))
		for i, n := range v {
			if n < 0 || n > 255 {
				return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("data[%d]=%d out of 0..255", i, n))
			}
			out[i] = byte(n)
		}
		if len(out) == 0 {
			return nil, model.NewAppError(model.ErrInvalidParams, "data is empty")
		}
		return out, nil
	case []float64:
		out := make([]byte, len(v))
		for i, f := range v {
			if f != float64(int(f)) || f < 0 || f > 255 {
				return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("data[%d]=%v out of 0..255", i, f))
			}
			out[i] = byte(f)
		}
		if len(out) == 0 {
			return nil, model.NewAppError(model.ErrInvalidParams, "data is empty")
		}
		return out, nil
	default:
		return nil, model.NewAppError(model.ErrInvalidParams, "data must be hex string or array of 0..255")
	}
}

func intsFromAny(v []interface{}) ([]byte, error) {
	if len(v) == 0 {
		return nil, model.NewAppError(model.ErrInvalidParams, "data is empty")
	}
	out := make([]byte, len(v))
	for i, item := range v {
		n, err := toByte(item)
		if err != nil {
			return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("data[%d]: %v", i, err))
		}
		out[i] = n
	}
	return out, nil
}

func toByte(v interface{}) (byte, error) {
	switch n := v.(type) {
	case int:
		if n < 0 || n > 255 {
			return 0, fmt.Errorf("%d out of 0..255", n)
		}
		return byte(n), nil
	case int64:
		if n < 0 || n > 255 {
			return 0, fmt.Errorf("%d out of 0..255", n)
		}
		return byte(n), nil
	case float64:
		if n != float64(int(n)) || n < 0 || n > 255 {
			return 0, fmt.Errorf("%v out of 0..255", n)
		}
		return byte(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil || i < 0 || i > 255 {
			return 0, fmt.Errorf("%v out of 0..255", n)
		}
		return byte(i), nil
	default:
		return 0, fmt.Errorf("want integer 0..255, got %T", v)
	}
}

// BytesToInts converts payload bytes to JSON-friendly 0..255 ints.
func BytesToInts(b []byte) []int {
	out := make([]int, len(b))
	for i, x := range b {
		out[i] = int(x)
	}
	return out
}

// BytesToHex returns lowercase hex without separators.
func BytesToHex(b []byte) string {
	return hex.EncodeToString(b)
}

// ValidateTarget checks ip/host and port.
func ValidateTarget(ip string, port int) (string, int, error) {
	host := strings.TrimSpace(ip)
	if host == "" {
		return "", 0, model.NewAppError(model.ErrInvalidParams, "ip is required")
	}
	if port < 1 || port > 65535 {
		return "", 0, model.NewAppError(model.ErrInvalidParams, "port must be 1..65535")
	}
	return host, port, nil
}

// ResolveTimeout parses optional duration and clamps to parent context deadline.
func ResolveTimeout(ctxDeadline time.Time, timeoutStr string, fallback time.Duration) (time.Duration, error) {
	d := fallback
	if strings.TrimSpace(timeoutStr) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(timeoutStr))
		if err != nil {
			return 0, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("timeout: %v", err))
		}
		if parsed <= 0 {
			return 0, model.NewAppError(model.ErrInvalidParams, "timeout must be positive")
		}
		d = parsed
	}
	if !ctxDeadline.IsZero() {
		remain := time.Until(ctxDeadline)
		if remain <= 0 {
			return 0, model.NewAppError(model.ErrConnectorError, "context deadline exceeded")
		}
		if remain < d {
			d = remain
		}
	}
	return d, nil
}

// ResolveMaxResponseBytes applies defaults and hard cap.
func ResolveMaxResponseBytes(n int) (int, error) {
	if n < 0 {
		return 0, model.NewAppError(model.ErrInvalidParams, "max_response_bytes must be >= 0")
	}
	if n == 0 {
		n = DefaultMaxResponseBytes
	}
	if n > AbsoluteMaxResponseBytes {
		return 0, model.NewAppError(model.ErrInvalidParams,
			fmt.Sprintf("max_response_bytes must be <= %d", AbsoluteMaxResponseBytes))
	}
	return n, nil
}

// JoinHostPort formats host:port for Dial (handles IPv6).
func JoinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
