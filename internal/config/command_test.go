package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoadCommandsAbsolutePathAndSummary(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: echo
    description: local echo
    path: /bin/echo
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hosts+`"
  databases: "`+filepath.Join(dir, "missing-dbs.yaml")+`"
  redis: "`+filepath.Join(dir, "missing-redis.yaml")+`"
  apis: "`+filepath.Join(dir, "missing-apis.yaml")+`"
  commands: "`+commands+`"
defaults:
  plugin_timeout: 15s
log:
  level: info
  encoding: console
`), 0o600)

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cmd, err := m.GetCommand("echo")
	if err != nil {
		t.Fatalf("GetCommand: %v", err)
	}
	if cmd.Path != "/bin/echo" {
		t.Fatalf("path=%q", cmd.Path)
	}
	sum := m.ListCommandSummaries()
	if len(sum) != 1 || sum[0].Name != "echo" {
		t.Fatalf("summaries=%+v", sum)
	}
	if _, err := m.GetCommand("missing"); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestLoadCommandsPathArrayPicksFirstAvailable(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	missing := filepath.Join(dir, "missing-bin")
	present := filepath.Join(dir, "present-bin")
	if err := os.WriteFile(present, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: tool
    description: multi-os paths
    path:
      - "`+missing+`"
      - "`+present+`"
      - /bin/echo
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

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cmd, err := m.GetCommand("tool")
	if err != nil {
		t.Fatalf("GetCommand: %v", err)
	}
	if cmd.Path != present {
		t.Fatalf("resolved path=%q want %q", cmd.Path, present)
	}
}

func TestLoadCommandsNoAvailablePathWarnsAndSkips(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	missingA := filepath.Join(dir, "a")
	missingB := filepath.Join(dir, "b")

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: gone
    path:
      - "`+missingA+`"
      - "`+missingB+`"
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

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := m.GetCommand("gone"); err == nil {
		t.Fatal("expected unavailable command to be skipped")
	}
	if len(m.ListCommandSummaries()) != 0 {
		t.Fatalf("summaries=%+v", m.ListCommandSummaries())
	}

	core, logs := observer.New(zap.WarnLevel)
	m.SetLogger(zap.New(core))
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("warnings=%d %+v", len(entries), entries)
	}
	if !strings.Contains(entries[0].Message, `command "gone" has no available path`) {
		t.Fatalf("msg=%q", entries[0].Message)
	}
}

func TestLoadCommandsRejectsRelativePath(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	commands := filepath.Join(dir, "commands.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(commands, []byte(`
commands:
  - name: bad
    path:
      - relative/bin
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

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected relative path to fail")
	}
}

func TestLoadCommandsMissingFileEmpty(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server: {host: "127.0.0.1", port: 20267}
plugins: {dir: "./plugins"}
config:
  hosts: "`+hosts+`"
  commands: "`+filepath.Join(dir, "no-commands.yaml")+`"
defaults: {plugin_timeout: 15s}
log: {level: info, encoding: console}
`), 0o600)

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.ListCommandSummaries()) != 0 {
		t.Fatal("expected empty commands")
	}
}

func TestRepoCommandsYAMLResolves(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	commandsYAML := filepath.Join(root, "config", "commands.yaml")
	if _, err := os.Stat(commandsYAML); err != nil {
		t.Skip("repo commands.yaml missing")
	}

	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server: {host: "127.0.0.1", port: 20267}
plugins: {dir: "./plugins"}
config:
  hosts: "`+hosts+`"
  commands: "`+commandsYAML+`"
defaults: {plugin_timeout: 15s}
log: {level: info, encoding: console}
`), 0o600)

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sum := m.ListCommandSummaries()
	if len(sum) == 0 {
		t.Fatal("expected at least one resolved command from repo commands.yaml")
	}
	for _, s := range sum {
		t.Logf("%s -> %s", s.Name, s.Path)
		if s.Path == "" || !filepath.IsAbs(s.Path) {
			t.Fatalf("%s bad path %q", s.Name, s.Path)
		}
	}
}
