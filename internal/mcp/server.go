package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/fsj00/ops-mcp/internal/openapi"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"go.uber.org/zap"
)

const ProtocolVersion = "2024-11-05"

// Server handles MCP JSON-RPC methods.
type Server struct {
	plugins  *plugin.Manager
	apiTools *openapi.Registry
	executor *executor.Executor
	log      *zap.Logger
}

func NewServer(plugins *plugin.Manager, exec *executor.Executor, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{
		plugins:  plugins,
		executor: exec,
		log:      log,
	}
}

// SetAPITools attaches the OpenAPI-generated tool registry.
func (s *Server) SetAPITools(reg *openapi.Registry) {
	s.apiTools = reg
}

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handle processes one JSON-RPC request. Notifications return nil response.
func (s *Server) Handle(ctx context.Context, raw []byte) (*Response, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &RPCError{Code: -32700, Message: "parse error"},
		}, nil
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return s.errResp(req.ID, -32600, "invalid request: jsonrpc must be 2.0", nil), nil
	}

	// Notifications have no id.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		return s.ok(req.ID, s.initialize(req.Params)), nil
	case "notifications/initialized", "initialized":
		return nil, nil
	case "ping":
		return s.ok(req.ID, map[string]interface{}{}), nil
	case "tools/list":
		return s.ok(req.ID, s.ToolsList()), nil
	case "tools/call":
		result := s.toolsCall(ctx, req.Params)
		return s.ok(req.ID, result), nil
	default:
		if isNotification {
			return nil, nil
		}
		return s.errResp(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method), nil), nil
	}
}

func (s *Server) initialize(params json.RawMessage) map[string]interface{} {
	protocol := ProtocolVersion
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(params, &p); err == nil && p.ProtocolVersion != "" {
		protocol = p.ProtocolVersion
	}
	return map[string]interface{}{
		"protocolVersion": protocol,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    "ops-mcp",
			"version": "0.1.0",
		},
	}
}

// ToolsList returns the merged MCP tool list (disk plugins + OpenAPI tools).
func (s *Server) ToolsList() map[string]interface{} {
	plugins := s.plugins.List()
	apiCount := 0
	if s.apiTools != nil {
		apiCount = s.apiTools.Count()
	}
	tools := make([]map[string]interface{}, 0, len(plugins)+apiCount)
	for _, p := range plugins {
		tools = append(tools, map[string]interface{}{
			"name":        p.Name,
			"description": p.Description,
			"inputSchema": p.InputSchema(),
		})
	}
	if s.apiTools != nil {
		for _, t := range s.apiTools.List() {
			tools = append(tools, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
	}
	return map[string]interface{}{"tools": tools}
}

// ToolsCount returns disk plugin count + OpenAPI tool count.
func (s *Server) ToolsCount() int {
	n := s.plugins.Count()
	if s.apiTools != nil {
		n += s.apiTools.Count()
	}
	return n
}

// APITools returns the OpenAPI registry (may be nil).
func (s *Server) APITools() *openapi.Registry {
	return s.apiTools
}

type callParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func (s *Server) toolsCall(ctx context.Context, params json.RawMessage) map[string]interface{} {
	var p callParams
	if err := json.Unmarshal(params, &p); err != nil {
		return toolErrorContent(model.FailResult(model.ErrInvalidParams, "invalid tools/call params"))
	}

	if pluginMeta, ok := s.plugins.Get(p.Name); ok {
		result := s.executor.Execute(ctx, pluginMeta, p.Arguments)
		return toolResultContent(result)
	}
	if s.apiTools != nil {
		if _, ok := s.apiTools.Get(p.Name); ok {
			result := s.apiTools.Call(ctx, p.Name, p.Arguments)
			return toolResultContent(result)
		}
	}
	return toolErrorContent(model.FailResult(model.ErrPluginNotFound, fmt.Sprintf("plugin %q not found", p.Name)))
}

func toolResultContent(result model.ToolResult) map[string]interface{} {
	text, _ := json.Marshal(result)
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(text)},
		},
		"isError": !result.Success,
	}
}

func toolErrorContent(result model.ToolResult) map[string]interface{} {
	return toolResultContent(result)
}

func (s *Server) ok(id json.RawMessage, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      decodeID(id),
		Result:  result,
	}
}

func (s *Server) errResp(id json.RawMessage, code int, message string, data interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      decodeID(id),
		Error:   &RPCError{Code: code, Message: message, Data: data},
	}
}

func decodeID(raw json.RawMessage) interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}
