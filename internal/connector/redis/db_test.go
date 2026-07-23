package redis

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"go.uber.org/zap"
)

func TestDialRejectsNegativeDB(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	rds := filepath.Join(dir, "redis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	mustWrite(t, hosts, []byte("hosts: []\n"))
	mustWrite(t, dbs, []byte("databases: []\n"))
	mustWrite(t, rds, []byte(`
redis:
  - name: local-redis
    connection:
      host: 127.0.0.1
      port: 6379
    readonly: true
`))
	mustWrite(t, cfgPath, []byte(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
  redis: "`+rds+`"
`))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	c := New(cfg, zap.NewNop())
	_, err = c.Ping(context.Background(), PingRequest{Redis: "local-redis", DB: -1})
	if err == nil {
		t.Fatal("expected error for negative db")
	}
}

func TestIntegrationRedisSelectDB(t *testing.T) {
	if os.Getenv("OPS_MCP_INTEGRATION") != "1" {
		t.Skip("set OPS_MCP_INTEGRATION=1 to run")
	}
	root := findRepoRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "config", "ops-mcp.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, err := cfg.GetRedis("local-redis"); err != nil {
		t.Skip("local-redis not configured")
	}

	c := New(cfg, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Seed via dial on db=1 indirectly: use Exists after writing with go-redis in test helper.
	// Prefer exercising connector: ping both db 0 and db 1.
	if _, err := c.Ping(ctx, PingRequest{Redis: "local-redis"}); err != nil {
		t.Fatalf("ping db0: %v", err)
	}
	if _, err := c.Ping(ctx, PingRequest{Redis: "local-redis", DB: 1}); err != nil {
		t.Fatalf("ping db1: %v", err)
	}

	client, _, err := c.dial(ctx, "local-redis", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Set(ctx, "ops-mcp:db-select", "on-db-1", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}

	ex0, err := c.Exists(ctx, KeyRequest{Redis: "local-redis", DB: 0, Key: "ops-mcp:db-select"})
	if err != nil {
		t.Fatal(err)
	}
	if ex0.Exists {
		t.Fatal("key should not exist on db 0")
	}
	ex1, err := c.Exists(ctx, KeyRequest{Redis: "local-redis", DB: 1, Key: "ops-mcp:db-select"})
	if err != nil {
		t.Fatal(err)
	}
	if !ex1.Exists {
		t.Fatal("key should exist on db 1")
	}
}
