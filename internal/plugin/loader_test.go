package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsj00/ops-mcp/internal/plugin"
)

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	pdir := filepath.Join(dir, "linux", "ls")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.yml"), []byte(`
name: linux_ls
version: "1.0"
description: list
type: command
target:
  type: ssh
input:
  path:
    type: string
    required: true
  host:
    type: string
    required: true
runtime: javascript
timeout: 10s
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "main.js"), []byte(`
function execute(ctx) { return { ok: true }; }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, errs := plugin.LoadAll(dir, nil)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(plugins) != 1 || plugins[0].Name != "linux_ls" {
		t.Fatalf("plugins=%v", plugins)
	}
	schema := plugins[0].InputSchema()
	req, _ := schema["required"].([]string)
	if len(req) != 2 {
		t.Fatalf("required=%v", req)
	}
}

func TestManagerLoad(t *testing.T) {
	root := filepath.Join("..", "..", "plugins")
	if _, err := os.Stat(root); err != nil {
		t.Skip("plugins dir not available")
	}
	m := plugin.NewManager(root, nil)
	n, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if n < 43 {
		t.Fatalf("expected >=43 plugins, got %d", n)
	}
	if _, ok := m.Get("linux_ls"); !ok {
		t.Fatal("linux_ls missing")
	}
	if _, ok := m.Get("redis_get"); !ok {
		t.Fatal("redis_get missing")
	}
	for _, name := range []string{"redis_lrange", "redis_hget", "redis_hmget", "redis_hscan"} {
		if _, ok := m.Get(name); !ok {
			t.Fatalf("%s missing", name)
		}
	}
	if _, ok := m.Get("linux_journalctl"); !ok {
		t.Fatal("linux_journalctl missing")
	}
	if _, ok := m.Get("linux_df"); !ok {
		t.Fatal("linux_df missing")
	}
	if _, ok := m.Get("linux_systemctl_failed"); !ok {
		t.Fatal("linux_systemctl_failed missing")
	}
	if _, ok := m.Get("docker_ps"); !ok {
		t.Fatal("docker_ps missing")
	}
	if _, ok := m.Get("docker_stats"); !ok {
		t.Fatal("docker_stats missing")
	}
	if _, ok := m.Get("docker_inspect"); !ok {
		t.Fatal("docker_inspect missing")
	}
	if _, ok := m.Get("linux_top"); !ok {
		t.Fatal("linux_top missing")
	}
	if _, ok := m.Get("list_hosts"); !ok {
		t.Fatal("list_hosts missing")
	}
}
