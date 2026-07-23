package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAPIsEnvExpandAndSummary(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	apis := filepath.Join(dir, "apis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	oa := filepath.Join(dir, "oa.yaml")

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(oa, []byte("openapi: 3.0.0\npaths: {}\n"), 0o600)
	t.Setenv("DEMO_TOKEN", "secret-token")
	_ = os.WriteFile(apis, []byte(`
apis:
  - name: demo
    description: demo api
    labels:
      env: dev
    openapi:
      path: "`+oa+`"
    endpoint:
      base_url: "http://example.com"
      timeout: 10s
    prefix: demo_
    headers:
      Authorization: "Bearer ${DEMO_TOKEN}"
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hosts+`"
  databases: "`+filepath.Join(dir, "db.yaml")+`"
  redis: "`+filepath.Join(dir, "redis.yaml")+`"
  apis: "`+apis+`"
defaults:
  plugin_timeout: 30s
log:
  level: error
  encoding: console
`), 0o600)

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := m.GetAPI("demo")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Headers["Authorization"] != "Bearer secret-token" {
		t.Fatalf("headers=%v", svc.Headers)
	}
	if !svc.Endpoint.VerifyTLSEnabled() {
		t.Fatal("verify_tls default should be true")
	}
	sum := m.ListAPISummaries()
	if len(sum) != 1 || sum[0].HasHeaders != true {
		t.Fatalf("summaries=%+v", sum)
	}
	// Ensure header value not present in summary JSON shape fields.
	if sum[0].BaseURL != "http://example.com" || sum[0].Prefix != "demo_" {
		t.Fatalf("summary=%+v", sum[0])
	}
}

func TestLoadAPIsMissingEnvFails(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	apis := filepath.Join(dir, "apis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	oa := filepath.Join(dir, "oa.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(oa, []byte("openapi: 3.0.0\npaths: {}\n"), 0o600)
	_ = os.Unsetenv("MISSING_API_TOKEN")
	_ = os.WriteFile(apis, []byte(`
apis:
  - name: demo
    openapi:
      path: "`+oa+`"
    endpoint:
      base_url: "http://example.com"
    headers:
      Authorization: "Bearer ${MISSING_API_TOKEN}"
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hosts+`"
  apis: "`+apis+`"
`), 0o600)

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestLoadAPIsMissingFileEmpty(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hosts+`"
  apis: "`+filepath.Join(dir, "no-apis.yaml")+`"
`), 0o600)
	m, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.ListAPISummaries()) != 0 {
		t.Fatal("expected empty apis")
	}
}
