package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
)

func TestExecEchoSuccess(t *testing.T) {
	cfg := loadTestConfig(t, "/bin/echo")
	c := New(cfg, nil)

	res, err := c.Exec(context.Background(), ExecRequest{
		Command: "echo",
		Args:    []string{"hello", "ops-mcp"},
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit=%d stderr=%q", res.ExitCode, res.Stderr)
	}
	if strings.TrimSpace(res.Stdout) != "hello ops-mcp" {
		t.Fatalf("stdout=%q", res.Stdout)
	}
}

func TestExecUnknownCommandInvalidParams(t *testing.T) {
	cfg := loadTestConfig(t, "/bin/echo")
	c := New(cfg, nil)

	_, err := c.Exec(context.Background(), ExecRequest{Command: "missing"})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("err=%v", err)
	}
}

func TestExecNonZeroExitStillReturnsResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t, script)
	c := New(cfg, nil)

	res, err := c.Exec(context.Background(), ExecRequest{Command: "echo"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("exit=%d", res.ExitCode)
	}
}

func TestExecTruncatesOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "big.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\npython3 -c 'print(\"x\"*200)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Prefer a tiny printf loop without python dependency.
	if err := os.WriteFile(script, []byte("#!/bin/sh\nawk 'BEGIN{for(i=0;i<200;i++)printf \"x\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t, script)
	c := New(cfg, nil)
	c.maxOutputBytes = 50

	res, err := c.Exec(context.Background(), ExecRequest{Command: "echo"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(res.Stdout, "...[truncated]") {
		t.Fatalf("expected truncation, got %q", res.Stdout)
	}
	if len(res.Stdout) > 50+len("\n...[truncated]") {
		t.Fatalf("stdout too long: %d", len(res.Stdout))
	}
}

func TestExecRespectsTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "sleep.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := loadTestConfig(t, script)
	c := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Exec(ctx, ExecRequest{Command: "echo"})
	if err == nil {
		t.Fatal("expected timeout")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrPluginTimeout {
		t.Fatalf("err=%v", err)
	}
}

func loadTestConfig(t *testing.T, binPath string) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: echo
    description: test binary
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
