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

func TestGetRequiresKey(t *testing.T) {
	c := New(nil, zap.NewNop())
	_, err := c.Get(context.Background(), KeyRequest{Redis: "local-redis", Key: "  "})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	app, ok := err.(*model.AppError)
	if !ok || app.Code != model.ErrInvalidParams {
		t.Fatalf("want INVALID_PARAMS, got %v", err)
	}
}

func TestIntegrationRedisGet(t *testing.T) {
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

	client, _, err := c.dial(ctx, "local-redis", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	const key = "ops-mcp:redis-get"
	if err := client.Set(ctx, key, "hello-get", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	got, err := c.Get(ctx, KeyRequest{Redis: "local-redis", Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value == nil || *got.Value != "hello-get" {
		t.Fatalf("unexpected get result: %+v", got)
	}

	missing, err := c.Get(ctx, KeyRequest{Redis: "local-redis", Key: "ops-mcp:redis-get-missing"})
	if err != nil {
		t.Fatal(err)
	}
	if missing.Value != nil {
		t.Fatalf("expected null value for missing key, got %q", *missing.Value)
	}
}
