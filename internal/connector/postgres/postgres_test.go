package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

func TestQueryRequiresDatabase(t *testing.T) {
	c := New(nil, zap.NewNop())
	_, err := c.Query(t.Context(), QueryRequest{SQL: "SELECT 1"})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("got %v", err)
	}
}

func TestQueryRejectsNonSelect(t *testing.T) {
	cfg := mustTempConfig(t, `databases:
  - name: local-postgres
    type: postgresql
    connection:
      host: 127.0.0.1
      port: 5432
      username: postgres
      password: "111111"
      database: postgres
      sslmode: disable
    readonly: true
    limit: 50
`)
	c := New(cfg, zap.NewNop())
	_, err := c.Query(t.Context(), QueryRequest{
		Database: "local-postgres",
		SQL:      "DELETE FROM pg_class",
	})
	if err == nil {
		t.Fatal("expected reject")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(ae.Message, "SELECT") {
		t.Fatalf("message=%q", ae.Message)
	}
}

func TestIntegrationLocalPostgres(t *testing.T) {
	if os.Getenv("OPS_MCP_INTEGRATION") != "1" {
		t.Skip("set OPS_MCP_INTEGRATION=1 to run")
	}
	cfg, err := config.Load("../../config/ops-mcp.yaml")
	if err != nil {
		// fall back to temp config using well-known local credentials
		cfg = mustTempConfig(t, `databases:
  - name: local-postgres
    type: postgresql
    connection:
      host: 127.0.0.1
      port: 5432
      username: postgres
      password: "111111"
      database: postgres
      sslmode: disable
    readonly: true
    limit: 100
`)
	}
	c := New(cfg, zap.NewNop())

	ver, err := c.Version(t.Context(), VersionRequest{Database: "local-postgres"})
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if ver == nil || !strings.Contains(strings.ToLower(ver.Version), "postgresql") {
		t.Fatalf("unexpected version: %+v", ver)
	}

	res, err := c.Query(t.Context(), QueryRequest{
		Database: "local-postgres",
		SQL:      "SELECT 1 AS n, 'ok' AS s",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.RowCount != 1 || len(res.Columns) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}

	_, err = c.Query(t.Context(), QueryRequest{
		Database: "missing-db",
		SQL:      "SELECT 1",
	})
	if err == nil {
		t.Fatal("expected missing database error")
	}
}

func mustTempConfig(t *testing.T, databasesYAML string) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	if err := os.WriteFile(hostsPath, []byte("hosts: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbsPath, []byte(databasesYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
defaults:
  plugin_timeout: 15s
log:
  level: info
  encoding: console
`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
