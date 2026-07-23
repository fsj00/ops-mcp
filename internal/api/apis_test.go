package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/api"
	"github.com/fsj00/ops-mcp/internal/config"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/openapi"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestListAPIsNoHeaderLeak(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	apis := filepath.Join(dir, "apis.yaml")
	oa := filepath.Join(dir, "oa.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(oa, []byte(`
openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "200": {description: ok}
`), 0o600)
	t.Setenv("API_TOKEN", "super-secret-token")
	_ = os.WriteFile(apis, []byte(`
apis:
  - name: demo
    description: demo
    openapi:
      path: "`+oa+`"
    endpoint:
      base_url: http://127.0.0.1:9
    prefix: demo_
    headers:
      Authorization: "Bearer ${API_TOKEN}"
    discovery:
      include:
        - operation_ids: ["^ping$"]
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18083
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  apis: "`+apis+`"
defaults:
  plugin_timeout: 5s
log:
  level: error
  encoding: console
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	pm := plugin.NewManager(plugins, nil)
	reg := openapi.NewRegistry(cfg, httpconn.New(cfg, nil), nil)
	reg.SetReservedNamesProvider(pm.Names)
	pm.SetReservedNamesProvider(reg.Names)
	if _, err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(runtime.Dependencies{Cfg: cfg, APIs: reg})
	ex := executor.New(rt, cfg.DefaultPluginTimeout())
	mcpServer := mcp.NewServer(pm, ex, nil)
	mcpServer.SetAPITools(reg)
	srv := api.New(cfg, pm, mcpServer, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/apis", nil)
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "super-secret-token") {
		t.Fatalf("token leaked: %s", body)
	}
	var resp struct {
		Count int `json:"count"`
		APIs  []struct {
			Name      string `json:"name"`
			ToolCount int    `json:"tool_count"`
			HasHeaders bool  `json:"has_headers"`
		} `json:"apis"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 || resp.APIs[0].Name != "demo" || resp.APIs[0].ToolCount != 1 || !resp.APIs[0].HasHeaders {
		t.Fatalf("resp=%+v", resp)
	}
}
