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

// Requires: OPS_MCP_INTEGRATION=1 and a reachable redis from redis.yaml (e.g. local docker).
func TestIntegrationRedisOps(t *testing.T) {
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
		t.Fatalf("local-redis not configured: %v", err)
	}

	c := New(cfg, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ping, err := c.Ping(ctx, PingRequest{Redis: "local-redis"})
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if ping.Result != "PONG" {
		t.Fatalf("ping result=%q", ping.Result)
	}

	info, err := c.Info(ctx, InfoRequest{Redis: "local-redis", Section: "server"})
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Info == "" {
		t.Fatal("empty info")
	}

	if _, err := c.Scan(ctx, ScanRequest{Redis: "local-redis", Limit: 0}); err == nil {
		t.Fatal("scan without limit should fail")
	}

	scan, err := c.Scan(ctx, ScanRequest{Redis: "local-redis", Match: "*", Limit: 10})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scan.Count > 10 {
		t.Fatalf("scan count=%d exceeds limit", scan.Count)
	}

	if _, err := c.ClientList(ctx, ClientListRequest{Redis: "local-redis", Limit: 0}); err == nil {
		t.Fatal("client_list without limit should fail")
	}
	clients, err := c.ClientList(ctx, ClientListRequest{Redis: "local-redis", Limit: 5})
	if err != nil {
		t.Fatalf("client_list: %v", err)
	}
	if clients.Count == 0 {
		t.Fatal("expected at least one client")
	}

	cfgGet, err := c.ConfigGet(ctx, ConfigGetRequest{Redis: "local-redis", Pattern: "maxmemory"})
	if err != nil {
		t.Fatalf("config_get: %v", err)
	}
	if len(cfgGet.Config) == 0 {
		t.Fatal("expected config entries")
	}

	if _, err := c.DBSize(ctx, DBSizeRequest{Redis: "local-redis"}); err != nil {
		t.Fatalf("dbsize: %v", err)
	}
	if _, err := c.Role(ctx, RoleRequest{Redis: "local-redis"}); err != nil {
		t.Fatalf("role: %v", err)
	}
	if _, err := c.SlowlogGet(ctx, SlowlogGetRequest{Redis: "local-redis", Count: 5}); err != nil {
		t.Fatalf("slowlog_get: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
