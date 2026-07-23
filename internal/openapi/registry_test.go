package openapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	"github.com/fsj00/ops-mcp/internal/openapi"
)

func TestRegistryLoadAndCall(t *testing.T) {
	dir := t.TempDir()
	openapiPath := filepath.Join(dir, "cmdb.yaml")
	apisPath := filepath.Join(dir, "apis.yaml")
	hostsPath := filepath.Join(dir, "hosts.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/host/42" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 42, "name": "web"})
	}))
	defer upstream.Close()

	_ = os.WriteFile(openapiPath, []byte(`
openapi: 3.0.3
info:
  title: CMDB
  version: "1.0"
paths:
  /host/{id}:
    get:
      operationId: getHostById
      summary: Get host by id
      tags: [host]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: ok
  /hosts:
    post:
      operationId: createHost
      summary: Create host
      responses:
        "200":
          description: ok
  /admin/secret:
    get:
      operationId: getSecret
      tags: [admin]
      responses:
        "200":
          description: ok
`), 0o600)

	t.Setenv("CMDB_TOKEN", "test-token")
	_ = os.WriteFile(apisPath, []byte(`
apis:
  - name: cmdb
    description: CMDB API Service
    openapi:
      path: "`+openapiPath+`"
    endpoint:
      base_url: "`+upstream.URL+`"
      timeout: 5s
      verify_tls: true
    prefix: cmdb_
    headers:
      Authorization: "Bearer ${CMDB_TOKEN}"
    discovery:
      include:
        - operation_ids:
            - "^get.*"
      exclude:
        - tags:
            - admin
        - methods:
            - POST
`), 0o600)
	_ = os.WriteFile(hostsPath, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18090
plugins:
  dir: "`+dir+`"
config:
  hosts: "`+hostsPath+`"
  databases: "`+filepath.Join(dir, "databases.yaml")+`"
  redis: "`+filepath.Join(dir, "redis.yaml")+`"
  apis: "`+apisPath+`"
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
	reg := openapi.NewRegistry(cfg, httpconn.New(cfg, nil), nil)
	n, err := reg.Load()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("tool count=%d want 1", n)
	}
	tool, ok := reg.Get("cmdb_getHostById")
	if !ok {
		t.Fatal("missing cmdb_getHostById")
	}
	if tool.Description != "Get host by id" {
		t.Fatalf("desc=%q", tool.Description)
	}
	if _, ok := reg.Get("cmdb_createHost"); ok {
		t.Fatal("createHost should be excluded")
	}
	if _, ok := reg.Get("cmdb_getSecret"); ok {
		t.Fatal("admin tag should be excluded")
	}

	summaries := reg.ListAPISummaries()
	if len(summaries) != 1 || summaries[0].ToolCount != 1 || summaries[0].HasHeaders != true {
		t.Fatalf("summaries=%+v", summaries)
	}
	if summaries[0].BaseURL != upstream.URL {
		t.Fatalf("base_url=%s", summaries[0].BaseURL)
	}

	result := reg.Call(context.Background(), "cmdb_getHostById", map[string]interface{}{"id": "42"})
	if !result.Success {
		t.Fatalf("call failed: %+v", result.Error)
	}
	data, _ := json.Marshal(result.Data)
	if !json.Valid(data) || !containsAll(string(data), `"status_code":200`, `"name":"web"`) {
		t.Fatalf("data=%s", data)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
