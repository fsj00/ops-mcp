package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/api"
	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/mcp"
	"github.com/fsj00/ops-mcp/internal/plugin"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestListHosts(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)

	_ = os.WriteFile(hosts, []byte(`
hosts:
  - name: dev-ssh-111
    description: demo host
    address:
      host: 192.168.44.111
      port: 22
    auth:
      type: password
      username: root
      password: "secret-host"
`), 0o600)
	_ = os.WriteFile(dbs, []byte("databases: []\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18083
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
defaults:
  plugin_timeout: 5s
log:
  level: error
  encoding: console
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	pm := plugin.NewManager(plugins, nil)
	if _, err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(runtime.Dependencies{Cfg: cfg})
	ex := executor.New(rt, cfg.DefaultPluginTimeout())
	srv := api.New(cfg, pm, mcp.NewServer(pm, ex, nil), nil)
	r := srv.Router()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hosts", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("hosts status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-host") {
		t.Fatalf("password leaked in /api/hosts: %s", body)
	}
	if strings.Contains(body, "private_key") {
		t.Fatalf("private_key field leaked in /api/hosts: %s", body)
	}
	var hostResp struct {
		Count int `json:"count"`
		Hosts []struct {
			Name     string `json:"name"`
			AuthType string `json:"auth_type"`
			Username string `json:"username"`
			Address  struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"address"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hostResp); err != nil {
		t.Fatal(err)
	}
	if hostResp.Count != 1 || hostResp.Hosts[0].Name != "dev-ssh-111" {
		t.Fatalf("unexpected hosts resp: %+v", hostResp)
	}
	if hostResp.Hosts[0].AuthType != "password" || hostResp.Hosts[0].Username != "root" {
		t.Fatalf("unexpected auth summary: %+v", hostResp.Hosts[0])
	}
	if hostResp.Hosts[0].Address.Host != "192.168.44.111" || hostResp.Hosts[0].Address.Port != 22 {
		t.Fatalf("unexpected address: %+v", hostResp.Hosts[0].Address)
	}
}

func TestListDatabasesAndRedis(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	rds := filepath.Join(dir, "redis.yaml")
	kfk := filepath.Join(dir, "kafka.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)

	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(dbs, []byte(`
databases:
  - name: local-postgres
    type: postgresql
    connection:
      host: 127.0.0.1
      port: 5432
      username: postgres
      password: "secret-db"
      database: postgres
    readonly: true
    limit: 1000
`), 0o600)
	_ = os.WriteFile(rds, []byte(`
redis:
  - name: local-redis
    description: demo
    connection:
      host: 127.0.0.1
      port: 6379
      username: readonly
      password: "secret-redis"
    readonly: true
    limit: 500
`), 0o600)
	_ = os.WriteFile(kfk, []byte(`
kafka:
  - name: local-kafka
    description: demo
    connection:
      brokers: ["127.0.0.1:9092"]
      sasl:
        mechanism: plain
        username: readonly
        password: "secret-kafka"
    readonly: true
    limit: 500
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18082
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
  redis: "`+rds+`"
  kafka: "`+kfk+`"
defaults:
  plugin_timeout: 5s
log:
  level: error
  encoding: console
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	pm := plugin.NewManager(plugins, nil)
	if _, err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(runtime.Dependencies{Cfg: cfg})
	ex := executor.New(rt, cfg.DefaultPluginTimeout())
	srv := api.New(cfg, pm, mcp.NewServer(pm, ex, nil), nil)
	r := srv.Router()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("databases status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-db") {
		t.Fatalf("password leaked in /api/databases: %s", body)
	}
	var dbResp struct {
		Count     int `json:"count"`
		Databases []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"databases"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &dbResp); err != nil {
		t.Fatal(err)
	}
	if dbResp.Count != 1 || dbResp.Databases[0].Name != "local-postgres" || dbResp.Databases[0].Type != "postgresql" {
		t.Fatalf("unexpected databases resp: %+v", dbResp)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/redis", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("redis status=%d body=%s", w.Code, w.Body.String())
	}
	body = w.Body.String()
	if strings.Contains(body, "secret-redis") {
		t.Fatalf("password leaked in /api/redis: %s", body)
	}
	var redisResp struct {
		Count int `json:"count"`
		Redis []struct {
			Name       string `json:"name"`
			Connection struct {
				Username string `json:"username"`
			} `json:"connection"`
		} `json:"redis"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &redisResp); err != nil {
		t.Fatal(err)
	}
	if redisResp.Count != 1 || redisResp.Redis[0].Name != "local-redis" {
		t.Fatalf("unexpected redis resp: %+v", redisResp)
	}
	if redisResp.Redis[0].Connection.Username != "readonly" {
		t.Fatalf("username=%q", redisResp.Redis[0].Connection.Username)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/kafka", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("kafka status=%d body=%s", w.Code, w.Body.String())
	}
	body = w.Body.String()
	if strings.Contains(body, "secret-kafka") {
		t.Fatalf("password leaked in /api/kafka: %s", body)
	}
	var kafkaResp struct {
		Count int `json:"count"`
		Kafka []struct {
			Name       string `json:"name"`
			Connection struct {
				SASL struct {
					Username string `json:"username"`
				} `json:"sasl"`
			} `json:"connection"`
		} `json:"kafka"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &kafkaResp); err != nil {
		t.Fatal(err)
	}
	if kafkaResp.Count != 1 || kafkaResp.Kafka[0].Name != "local-kafka" {
		t.Fatalf("unexpected kafka resp: %+v", kafkaResp)
	}
	if kafkaResp.Kafka[0].Connection.SASL.Username != "readonly" {
		t.Fatalf("sasl username=%q", kafkaResp.Kafka[0].Connection.SASL.Username)
	}
}
