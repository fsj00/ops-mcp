package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Connector performs outbound HTTP calls (apis.yaml by name, or absolute URL).
type Connector struct {
	cfg *config.Manager
	log *zap.Logger
}

func New(cfg *config.Manager, log *zap.Logger) *Connector {
	if log == nil {
		log = zap.NewNop()
	}
	return &Connector{cfg: cfg, log: log}
}

// Request is an outbound HTTP request.
//
// Two modes (mutually exclusive):
//   - API mode: set API (+ Path relative to apis.yaml base_url)
//   - URL mode: set URL (absolute); Path ignored
type Request struct {
	API    string // apis.yaml service name
	URL    string // absolute URL (api mode and url mode are mutually exclusive)
	Method string
	Path   string // API mode: path under base_url
	Query  map[string]string
	Headers map[string]string
	Body   interface{} // encoded as JSON when non-nil
	Timeout time.Duration
	// VerifyTLS overrides TLS verification in URL mode; nil = true.
	// In API mode, apis.yaml endpoint.verify_tls is used unless this is set.
	VerifyTLS *bool
}

// Result is returned to MCP tools / JS plugins.
type Result struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       interface{}       `json:"body"`
}

// Do executes the HTTP request.
func (c *Connector) Do(ctx context.Context, req Request) (*Result, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		return nil, model.NewAppError(model.ErrInvalidParams, "method is required")
	}
	hasAPI := strings.TrimSpace(req.API) != ""
	hasURL := strings.TrimSpace(req.URL) != ""
	if hasAPI && hasURL {
		return nil, model.NewAppError(model.ErrInvalidParams, "api and url are mutually exclusive")
	}
	if !hasAPI && !hasURL {
		return nil, model.NewAppError(model.ErrInvalidParams, "either api or url is required")
	}

	var (
		targetURL      *url.URL
		baseHeaders    map[string]string
		timeout        time.Duration
		verifyTLS      = true
		logAPI         string
	)

	if hasAPI {
		svc, err := c.cfg.GetAPI(req.API)
		if err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, err.Error())
		}
		if strings.TrimSpace(req.Path) == "" {
			return nil, model.NewAppError(model.ErrInvalidParams, "path is required when using api")
		}
		base := strings.TrimRight(svc.Endpoint.BaseURL, "/")
		path := req.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		u, err := url.Parse(base + path)
		if err != nil {
			return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("invalid url: %v", err))
		}
		targetURL = u
		baseHeaders = svc.Headers
		timeout = req.Timeout
		if timeout <= 0 {
			timeout = svc.Endpoint.TimeoutDuration()
		}
		verifyTLS = svc.Endpoint.VerifyTLSEnabled()
		if req.VerifyTLS != nil {
			verifyTLS = *req.VerifyTLS
		}
		logAPI = req.API
	} else {
		u, err := url.Parse(req.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, model.NewAppError(model.ErrInvalidParams, "url must be an absolute http(s) URL")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, model.NewAppError(model.ErrInvalidParams, "url scheme must be http or https")
		}
		targetURL = u
		timeout = req.Timeout
		if req.VerifyTLS != nil {
			verifyTLS = *req.VerifyTLS
		}
		logAPI = ""
	}

	if timeout <= 0 && c.cfg != nil {
		timeout = c.cfg.DefaultPluginTimeout()
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	q := targetURL.Query()
	for k, v := range req.Query {
		q.Set(k, v)
	}
	targetURL.RawQuery = q.Encode()

	var bodyReader io.Reader
	if req.Body != nil {
		raw, err := json.Marshal(req.Body)
		if err != nil {
			return nil, model.NewAppError(model.ErrInvalidParams, fmt.Sprintf("encode body: %v", err))
		}
		bodyReader = bytes.NewReader(raw)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, targetURL.String(), bodyReader)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, err.Error())
	}
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for k, v := range baseHeaders {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !verifyTLS, //nolint:gosec // intentional for verify_tls:false
			},
		},
	}

	c.log.Info("http request",
		zap.String("api", logAPI),
		zap.String("method", method),
		zap.String("url", redactURL(targetURL)),
	)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("http request failed: %v", err))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return nil, model.NewAppError(model.ErrConnectorError, fmt.Sprintf("read response: %v", err))
	}

	headers := map[string]string{}
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[strings.ToLower(k)] = vals[0]
		}
	}

	var body interface{}
	ct := resp.Header.Get("Content-Type")
	if len(raw) == 0 {
		body = nil
	} else if strings.Contains(ct, "json") || json.Valid(raw) {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			body = string(raw)
		} else {
			body = v
		}
	} else {
		body = string(raw)
	}

	c.log.Info("http response",
		zap.String("api", logAPI),
		zap.String("method", method),
		zap.Int("status_code", resp.StatusCode),
	)

	return &Result{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

func redactURL(u *url.URL) string {
	cp := *u
	cp.User = nil
	return cp.String()
}
