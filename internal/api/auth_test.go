package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/api"
	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestAuthMiddleware(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(filepath.Join(plugins, "t"), 0o755)
	_ = os.WriteFile(filepath.Join(plugins, "t", "plugin.yml"), []byte(`
name: t
version: "1.0"
description: t
type: command
target:
  type: ssh
input: {}
runtime: javascript
`), 0o644)
	_ = os.WriteFile(filepath.Join(plugins, "t", "main.js"), []byte(`function execute(ctx){return {};}`), 0o644)
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(dbs, []byte("databases: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18081
  auth:
    token: "secret-token"
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
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
	if _, err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(runtime.Dependencies{})
	ex := executor.New(rt, cfg.DefaultPluginTimeout())
	srv := api.New(cfg, pm, mcp.NewServer(pm, ex, nil), nil)
	r := srv.Router()

	// no token -> 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}

	// wrong token -> 401
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}

	// correct bearer -> 200
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// health remains public
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health want 200, got %d", w.Code)
	}

	// embedded UI remains public
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET / want 200, got %d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / Content-Type=%q want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), `data-ops-mcp-ui="1"`) {
		t.Fatalf("GET / body missing UI marker")
	}
}
