package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	cmdconn "github.com/fsj00/ops-mcp/internal/connector/command"
)

func TestCommandExecBridge(t *testing.T) {
	cfg := loadCommandRuntimeConfig(t, "/bin/echo")
	rt := New(Dependencies{
		Command: cmdconn.New(cfg, nil),
		Cfg:     cfg,
	})

	script := `
function execute(ctx) {
  return ctx.command.exec({ command: "echo", args: ["bridge-ok"] });
}
`
	val, err := rt.Execute(context.Background(), script, map[string]interface{}{}, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("type=%T", val)
	}
	stdout, _ := m["stdout"].(string)
	if strings.TrimSpace(stdout) != "bridge-ok" {
		t.Fatalf("stdout=%v", m["stdout"])
	}
}

func TestCommandsListBridge(t *testing.T) {
	cfg := loadCommandRuntimeConfig(t, "/bin/echo")
	rt := New(Dependencies{
		Command: cmdconn.New(cfg, nil),
		Cfg:     cfg,
	})

	script := `
function execute(ctx) {
  var list = ctx.commands.list();
  return { count: list.length, name: list[0].name };
}
`
	val, err := rt.Execute(context.Background(), script, map[string]interface{}{}, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("type=%T", val)
	}
	switch n := m["count"].(type) {
	case int64:
		if n != 1 {
			t.Fatalf("count=%v", m["count"])
		}
	case int:
		if n != 1 {
			t.Fatalf("count=%v", m["count"])
		}
	case float64:
		if n != 1 {
			t.Fatalf("count=%v", m["count"])
		}
	default:
		t.Fatalf("count type=%T val=%v", m["count"], m["count"])
	}
	if m["name"] != "echo" {
		t.Fatalf("name=%v", m["name"])
	}
}

func loadCommandRuntimeConfig(t *testing.T, binPath string) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: echo
    path: "`+binPath+`"
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server: {host: "127.0.0.1", port: 20267}
plugins: {dir: "./plugins"}
config:
  hosts: "`+hosts+`"
  commands: "`+commands+`"
defaults: {plugin_timeout: 15s}
log: {level: info, encoding: console}
`), 0o600)
	m, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return m
}
