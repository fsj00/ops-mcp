package openapi

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fsj00/ops-mcp/internal/config"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Registry holds OpenAPI-generated MCP tools.
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]*ToolMeta
	byAPI    map[string]int
	cfg      *config.Manager
	http     *httpconn.Connector
	log      *zap.Logger
	reserved func() []string // disk plugin names for conflict checks on reload
}

// NewRegistry creates an empty registry.
func NewRegistry(cfg *config.Manager, httpC *httpconn.Connector, log *zap.Logger) *Registry {
	if log == nil {
		log = zap.NewNop()
	}
	return &Registry{
		tools: map[string]*ToolMeta{},
		byAPI: map[string]int{},
		cfg:   cfg,
		http:  httpC,
		log:   log,
	}
}

// SetReservedNamesProvider supplies disk plugin names for conflict detection.
func (r *Registry) SetReservedNamesProvider(fn func() []string) {
	r.reserved = fn
}

// Load builds tools from configured API services.
// On failure, the previous tool set is retained.
func (r *Registry) Load() (int, error) {
	apis := r.cfg.ListAPIs()
	newTools := map[string]*ToolMeta{}
	newByAPI := map[string]int{}

	reserved := map[string]struct{}{}
	if r.reserved != nil {
		for _, n := range r.reserved() {
			reserved[n] = struct{}{}
		}
	}

	for _, svc := range apis {
		path := svc.OpenAPI.Path
		if !filepath.IsAbs(path) {
			// relative to process working directory (documented)
		}
		doc, err := loadDoc(path)
		if err != nil {
			return 0, err
		}
		disc, err := NewDiscoveryMatcher(svc.Discovery)
		if err != nil {
			return 0, fmt.Errorf("api %q: %w", svc.Name, err)
		}
		ops, warns := extractOperations(svc.Name, doc)
		for _, w := range warns {
			r.log.Warn("openapi", zap.String("api", svc.Name), zap.String("msg", w))
		}
		seenLocal := map[string]struct{}{}
		count := 0
		for _, op := range ops {
			if op.SkipReason == "duplicate_operation_id" {
				return 0, fmt.Errorf("api %q: duplicate operationId %q", svc.Name, op.OperationID)
			}
			if op.SkipReason != "" {
				r.log.Warn("openapi skip operation",
					zap.String("api", svc.Name),
					zap.String("operationId", op.OperationID),
					zap.String("reason", op.SkipReason),
				)
				continue
			}
			if !disc.Match(op) {
				continue
			}
			if _, dup := seenLocal[op.OperationID]; dup {
				return 0, fmt.Errorf("api %q: duplicate operationId %q", svc.Name, op.OperationID)
			}
			seenLocal[op.OperationID] = struct{}{}

			meta, tw := buildToolMeta(svc.Prefix, op)
			for _, w := range tw {
				r.log.Warn("openapi", zap.String("api", svc.Name), zap.String("msg", w))
			}
			if _, exists := newTools[meta.Name]; exists {
				return 0, fmt.Errorf("duplicate openapi tool name %q", meta.Name)
			}
			if _, clash := reserved[meta.Name]; clash {
				return 0, fmt.Errorf("openapi tool %q conflicts with disk plugin", meta.Name)
			}
			newTools[meta.Name] = meta
			count++
		}
		newByAPI[svc.Name] = count
	}

	r.mu.Lock()
	r.tools = newTools
	r.byAPI = newByAPI
	r.mu.Unlock()
	return len(newTools), nil
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (*ToolMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns tools sorted by name.
func (r *Registry) List() []*ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ToolMeta, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Count returns the number of registered OpenAPI tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// CountForAPI returns exposed tool count for one API service.
func (r *Registry) CountForAPI(name string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byAPI[name]
}

// ListAPISummaries returns API summaries with tool_count filled.
func (r *Registry) ListAPISummaries() []model.APISummary {
	base := r.cfg.ListAPISummaries()
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := range base {
		base[i].ToolCount = r.byAPI[base[i].Name]
	}
	return base
}

// Names returns all OpenAPI tool names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for n := range r.tools {
		out = append(out, n)
	}
	return out
}

// Call executes an OpenAPI tool.
func (r *Registry) Call(ctx context.Context, name string, args map[string]interface{}) model.ToolResult {
	tool, ok := r.Get(name)
	if !ok {
		return model.FailResult(model.ErrPluginNotFound, fmt.Sprintf("tool %q not found", name))
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := validateArgs(tool, args); err != nil {
		return model.FailResult(model.ErrInvalidParams, err.Error())
	}

	path, err := fillPath(tool.Path, args)
	if err != nil {
		return model.FailResult(model.ErrInvalidParams, err.Error())
	}

	query := map[string]string{}
	headers := map[string]string{}
	for name, in := range tool.ParamIns {
		if in == "body" {
			continue
		}
		v, ok := args[name]
		if !ok || v == nil {
			continue
		}
		s := fmt.Sprint(v)
		switch in {
		case "query":
			query[name] = s
		case "header":
			headers[name] = s
		}
	}

	var body interface{}
	switch tool.BodyMode {
	case "body":
		body = args["body"]
	case "expand":
		obj := map[string]interface{}{}
		for name := range tool.BodyPropNames {
			if v, ok := args[name]; ok {
				obj[name] = v
			}
		}
		if len(obj) > 0 || tool.BodyRequired {
			body = obj
		}
	}

	svc, err := r.cfg.GetAPI(tool.APIName)
	if err != nil {
		return model.FailResult(model.ErrConnectorError, err.Error())
	}
	timeout := svc.Endpoint.TimeoutDuration()
	if timeout <= 0 {
		timeout = r.cfg.DefaultPluginTimeout()
	}

	res, err := r.http.Do(ctx, httpconn.Request{
		API:     tool.APIName,
		Method:  tool.Method,
		Path:    path,
		Query:   query,
		Headers: headers,
		Body:    body,
		Timeout: timeout,
	})
	if err != nil {
		if ae, ok := err.(*model.AppError); ok {
			return model.FailResult(ae.Code, ae.Message)
		}
		return model.FailResult(model.ErrConnectorError, err.Error())
	}
	return model.SuccessResult(res)
}

func validateArgs(tool *ToolMeta, args map[string]interface{}) error {
	schema := tool.InputSchema
	req, _ := schema["required"].([]string)
	// required may be []interface{} from JSON roundtrip — handle both
	if req == nil {
		if raw, ok := schema["required"].([]interface{}); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					req = append(req, s)
				}
			}
		}
	}
	for _, name := range req {
		v, ok := args[name]
		if !ok || v == nil {
			return fmt.Errorf("missing required parameter %q", name)
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" && tool.ParamIns[name] == "path" {
			return fmt.Errorf("missing required parameter %q", name)
		}
	}
	return nil
}
