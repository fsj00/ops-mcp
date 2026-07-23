package redis

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

func TestLRangeRequiresLimit(t *testing.T) {
	dir := t.TempDir()
	cfg := mustLoadMinimalRedisCfg(t, dir)
	c := New(cfg, zap.NewNop())
	_, err := c.LRange(context.Background(), LRangeRequest{Redis: "local-redis", Key: "list", Limit: 0})
	if err == nil {
		t.Fatal("expected error for limit=0")
	}
	app, ok := err.(*model.AppError)
	if !ok || app.Code != model.ErrInvalidParams {
		t.Fatalf("want INVALID_PARAMS, got %v", err)
	}
}

func TestHGetRequiresField(t *testing.T) {
	c := New(nil, zap.NewNop())
	_, err := c.HGet(context.Background(), HGetRequest{Redis: "local-redis", Key: "h", Field: " "})
	if err == nil {
		t.Fatal("expected error for empty field")
	}
}

func TestHMGetRequiresFields(t *testing.T) {
	c := New(nil, zap.NewNop())
	_, err := c.HMGet(context.Background(), HMGetRequest{Redis: "local-redis", Key: "h", Fields: nil})
	if err == nil {
		t.Fatal("expected error for empty fields")
	}
}

func TestHScanRequiresLimit(t *testing.T) {
	dir := t.TempDir()
	cfg := mustLoadMinimalRedisCfg(t, dir)
	c := New(cfg, zap.NewNop())
	_, err := c.HScan(context.Background(), HScanRequest{Redis: "local-redis", Key: "h", Limit: 0})
	if err == nil {
		t.Fatal("expected error for limit=0")
	}
}

func TestIntegrationRedisHashAndList(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, _, err := c.dial(ctx, "local-redis", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	const listKey = "ops-mcp:lrange"
	const hashKey = "ops-mcp:hash"
	_ = client.Del(ctx, listKey, hashKey)
	t.Cleanup(func() { _ = client.Del(context.Background(), listKey, hashKey).Err() })

	if err := client.RPush(ctx, listKey, "a", "b", "c", "d").Err(); err != nil {
		t.Fatal(err)
	}
	lr, err := c.LRange(ctx, LRangeRequest{Redis: "local-redis", Key: listKey, Start: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if lr.Count != 2 || lr.Values[0] != "b" || lr.Values[1] != "c" {
		t.Fatalf("unexpected lrange: %+v", lr)
	}

	if err := client.HSet(ctx, hashKey, "f1", "v1", "f2", "v2", "f3", "v3").Err(); err != nil {
		t.Fatal(err)
	}
	hg, err := c.HGet(ctx, HGetRequest{Redis: "local-redis", Key: hashKey, Field: "f1"})
	if err != nil {
		t.Fatal(err)
	}
	if hg.Value == nil || *hg.Value != "v1" {
		t.Fatalf("unexpected hget: %+v", hg)
	}
	missing, err := c.HGet(ctx, HGetRequest{Redis: "local-redis", Key: hashKey, Field: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if missing.Value != nil {
		t.Fatal("expected null for missing field")
	}

	hm, err := c.HMGet(ctx, HMGetRequest{Redis: "local-redis", Key: hashKey, Fields: []string{"f1", "nope", "f3"}})
	if err != nil {
		t.Fatal(err)
	}
	if hm.Count != 3 || hm.Fields[0].Value == nil || *hm.Fields[0].Value != "v1" || hm.Fields[1].Value != nil {
		t.Fatalf("unexpected hmget: %+v", hm)
	}

	hs, err := c.HScan(ctx, HScanRequest{Redis: "local-redis", Key: hashKey, Match: "f*", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if hs.Count < 3 {
		t.Fatalf("unexpected hscan count: %+v", hs)
	}
}

func mustLoadMinimalRedisCfg(t *testing.T, dir string) *config.Manager {
	t.Helper()
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
    limit: 100
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
	return cfg
}
