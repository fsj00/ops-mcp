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
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestListSNMP(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	snmp := filepath.Join(dir, "snmp.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(dbs, []byte("databases: []\n"), 0o600)
	_ = os.WriteFile(snmp, []byte(`
credentials:
  - name: shared
    version: 2c
    community: "secret-snmp-community"
devices:
  - name: sw-a
    labels: { site: dc1 }
    address: { host: 10.0.0.1 }
    credential: shared
  - name: sw-b
    labels: { site: lab }
    address: { host: 10.0.0.2 }
    auth:
      version: 2c
      community: "lab-secret"
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18084
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
  snmp: "`+snmp+`"
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
	rt := runtime.New(runtime.Dependencies{Cfg: cfg})
	ex := executor.New(rt, cfg.DefaultPluginTimeout())
	srv := api.New(cfg, pm, mcp.NewServer(pm, ex, nil), nil)
	r := srv.Router()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/snmp", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-snmp-community") || strings.Contains(body, "lab-secret") {
		t.Fatalf("secret leaked: %s", body)
	}
	var resp struct {
		Count   int `json:"count"`
		Devices []struct {
			Name       string `json:"name"`
			AuthMode   string `json:"auth_mode"`
			Credential string `json:"credential"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("count=%d", resp.Count)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/snmp?label=site=dc1", nil)
	r.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 || resp.Devices[0].Name != "sw-a" || resp.Devices[0].AuthMode != "credential" {
		t.Fatalf("filtered=%+v", resp)
	}
}
