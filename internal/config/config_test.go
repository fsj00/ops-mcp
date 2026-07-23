package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndGetHost(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	redisPath := filepath.Join(dir, "redis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	if err := os.WriteFile(hostsPath, []byte(`
hosts:
  - name: demo
    description: demo host
    labels:
      env: dev
      role: bastion
    address:
      host: 127.0.0.1
      port: 22
    auth:
      type: password
      username: root
      password: "secret"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbsPath, []byte(`databases: []`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(redisPath, []byte(`redis: []`), 0o600); err != nil {
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
  redis: "`+redisPath+`"
defaults:
  plugin_timeout: 15s
log:
  level: info
  encoding: console
`), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	h, err := m.GetHost("demo")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if h.Address.Host != "127.0.0.1" || h.Auth.Password != "secret" {
		t.Fatalf("unexpected host: %+v", h)
	}
	summaries := m.ListHostSummaries()
	if len(summaries) != 1 || summaries[0].Name != "demo" || summaries[0].AuthType != "password" {
		t.Fatalf("summaries=%+v", summaries)
	}
	if summaries[0].Username != "root" {
		t.Fatalf("username=%s", summaries[0].Username)
	}
	if summaries[0].Labels["env"] != "dev" || summaries[0].Labels["role"] != "bastion" {
		t.Fatalf("labels=%+v", summaries[0].Labels)
	}
	if _, err := m.GetHost("missing"); err == nil {
		t.Fatal("expected missing host error")
	}
	if m.DefaultPluginTimeout().Seconds() != 15 {
		t.Fatalf("timeout = %v", m.DefaultPluginTimeout())
	}
}

func TestLoadDatabaseDefaultLimit(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	if err := os.WriteFile(hostsPath, []byte(`hosts: []`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbsPath, []byte(`
databases:
  - name: local-postgres
    description: local postgres (dev)
    labels:
      env: dev
    type: postgresql
    connection:
      host: 127.0.0.1
      username: postgres
      password: "x"
      database: postgres
    readonly: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	db, err := m.GetDatabase("local-postgres")
	if err != nil {
		t.Fatal(err)
	}
	if db.Limit != DefaultQueryLimit {
		t.Fatalf("limit=%d want %d", db.Limit, DefaultQueryLimit)
	}
	if db.Connection.Port != 5432 {
		t.Fatalf("port=%d", db.Connection.Port)
	}
	if db.Description != "local postgres (dev)" || db.Labels["env"] != "dev" {
		t.Fatalf("description/labels unexpected: %+v", db)
	}
	dbSum := m.ListDatabaseSummaries()
	if len(dbSum) != 1 || dbSum[0].Description != "local postgres (dev)" || dbSum[0].Labels["env"] != "dev" {
		t.Fatalf("database summaries=%+v", dbSum)
	}
}

func TestLoadRedisDefaultsAndAuth(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	redisPath := filepath.Join(dir, "redis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")

	if err := os.WriteFile(hostsPath, []byte(`hosts: []`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbsPath, []byte(`databases: []`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(redisPath, []byte(`
redis:
  - name: local-redis
    description: local redis
    labels:
      env: staging
    connection:
      host: 127.0.0.1
      password: "secret"
      username: readonly
      tls:
        enabled: true
        server_name: redis.local
        ca_file: /etc/certs/ca.crt
        cert_file: /etc/certs/client.crt
        private_key_file: /etc/certs/client.key
    readonly: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
  redis: "`+redisPath+`"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r, err := m.GetRedis("local-redis")
	if err != nil {
		t.Fatal(err)
	}
	if r.Limit != DefaultQueryLimit {
		t.Fatalf("limit=%d want %d", r.Limit, DefaultQueryLimit)
	}
	if r.Connection.Port != 6379 {
		t.Fatalf("port=%d", r.Connection.Port)
	}
	if r.Connection.Password != "secret" || r.Connection.Username != "readonly" {
		t.Fatalf("auth fields unexpected: %+v", r.Connection)
	}
	if _, err := m.GetRedis("missing"); err == nil {
		t.Fatal("expected missing redis error")
	}
	summaries := m.ListRedisSummaries()
	if len(summaries) != 1 || summaries[0].Name != "local-redis" {
		t.Fatalf("redis summaries=%+v", summaries)
	}
	if summaries[0].Description != "local redis" || summaries[0].Labels["env"] != "staging" {
		t.Fatalf("redis summary meta=%+v", summaries[0])
	}
	if summaries[0].Connection.Username != "readonly" || summaries[0].Connection.Port != 6379 {
		t.Fatalf("redis summary connection=%+v", summaries[0].Connection)
	}
	tlsSum := summaries[0].Connection.TLS
	if !tlsSum.Enabled || !tlsSum.HasCA || !tlsSum.HasClientCert || tlsSum.ServerName != "redis.local" {
		t.Fatalf("redis tls summary=%+v", tlsSum)
	}
	dbs := m.ListDatabaseSummaries()
	if len(dbs) != 0 {
		t.Fatalf("expected empty databases, got %+v", dbs)
	}
}
