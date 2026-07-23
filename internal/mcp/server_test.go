package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestToolsListAndCall(t *testing.T) {
	root := filepath.Join("..", "..", "plugins")
	if _, err := os.Stat(root); err != nil {
		t.Skip("plugins dir not available")
	}
	pm := plugin.NewManager(root, nil)
	if _, err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(runtime.Dependencies{})
	ex := executor.New(rt, time.Second)
	s := mcp.NewServer(pm, ex, nil)

	listRaw, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	resp, err := s.Handle(context.Background(), listRaw)
	if err != nil || resp.Error != nil {
		t.Fatalf("list: err=%v resp=%+v", err, resp)
	}
	result := resp.Result.(map[string]interface{})
	toolsRaw, ok := result["tools"].([]interface{})
	if !ok {
		// Handle typed slice from direct Go construction.
		if tools, ok2 := result["tools"].([]map[string]interface{}); ok2 {
			if len(tools) < 13 {
				t.Fatalf("tools=%d", len(tools))
			}
			return
		}
		t.Fatalf("unexpected tools type: %T", result["tools"])
	}
	if len(toolsRaw) < 13 {
		t.Fatalf("tools=%d", len(toolsRaw))
	}

	initRaw, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "test", "version": "1"},
		},
	})
	initResp, err := s.Handle(context.Background(), initRaw)
	if err != nil || initResp.Error != nil {
		t.Fatalf("init: %v %+v", err, initResp)
	}
}
