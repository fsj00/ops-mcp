package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsj00/ops-mcp/internal/api"
	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestListCommands(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: ping
    description: local ping
    path: /sbin/ping
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 20267
  auth:
    token: ""
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  commands: "`+commands+`"
defaults:
  plugin_timeout: 15s
log:
  level: info
  encoding: console
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pm := plugin.NewManager(plugins, nil)
	if _, err := pm.Load(); err != nil {
		t.Fatalf("Load plugins: %v", err)
	}
	rt := runtime.New(runtime.Dependencies{Cfg: cfg})
	exec := executor.New(rt, cfg.DefaultPluginTimeout())
	mcpServer := mcp.NewServer(pm, exec, nil)
	srv := api.New(cfg, pm, mcpServer, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/commands", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Count    int `json:"count"`
		Commands []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Commands) != 1 || body.Commands[0].Name != "ping" {
		t.Fatalf("body=%+v", body)
	}
}
