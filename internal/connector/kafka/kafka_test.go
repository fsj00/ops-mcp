package kafka_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	kafkaconn "github.com/fsj00/ops-mcp/internal/connector/kafka"
	"github.com/fsj00/ops-mcp/internal/model"
)

func TestExecuteValidation(t *testing.T) {
	cfg := loadTestConfig(t, `
kafka:
  - name: local-kafka
    description: demo
    connection:
      brokers: ["127.0.0.1:19092"]
    readonly: true
    limit: 100
`)
	c := kafkaconn.New(cfg, nil)
	ctx := context.Background()

	if _, err := c.Execute(ctx, "", map[string]interface{}{"kafka": "local-kafka"}); err == nil {
		t.Fatal("expected error for empty action")
	} else if ae, ok := err.(*model.AppError); !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("want INVALID_PARAMS, got %v", err)
	}

	if _, err := c.Execute(ctx, "cluster_info", nil); err == nil {
		t.Fatal("expected error for missing kafka")
	}

	if _, err := c.Execute(ctx, "no_such_action", map[string]interface{}{"kafka": "local-kafka"}); err == nil {
		t.Fatal("expected unknown action error")
	} else if !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("unexpected: %v", err)
	}

	if _, err := c.Execute(ctx, "topic_detail", map[string]interface{}{"kafka": "local-kafka"}); err == nil {
		t.Fatal("expected missing topic")
	}

	if _, err := c.Execute(ctx, "consumer_lag", map[string]interface{}{"kafka": "local-kafka"}); err == nil {
		t.Fatal("expected missing group")
	}

	if _, err := c.Execute(ctx, "cluster_info", map[string]interface{}{"kafka": "missing"}); err == nil {
		t.Fatal("expected missing instance")
	} else if ae, ok := err.(*model.AppError); !ok || ae.Code != model.ErrConnectorError {
		t.Fatalf("want CONNECTOR_ERROR, got %v", err)
	}
}

func TestExecuteConnectFailure(t *testing.T) {
	cfg := loadTestConfig(t, `
kafka:
  - name: local-kafka
    description: unreachable
    connection:
      brokers: ["127.0.0.1:1"]
    readonly: true
`)
	c := kafkaconn.New(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.Execute(ctx, kafkaconn.ActionClusterInfo, map[string]interface{}{"kafka": "local-kafka"})
	if err == nil {
		t.Fatal("expected connect error")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrConnectorError {
		t.Fatalf("want CONNECTOR_ERROR, got %v", err)
	}
	if strings.Contains(err.Error(), "password") {
		t.Fatalf("password leaked: %v", err)
	}
}

func TestToSummaryStripsSecrets(t *testing.T) {
	inst := model.KafkaInstance{
		Name: "prod",
		Connection: model.KafkaConnection{
			Brokers: []string{"kafka:9093"},
			SASL: model.KafkaSASL{
				Mechanism: "scram-sha-256",
				Username:  "ro",
				Password:  "secret-kafka",
			},
			TLS: model.KafkaTLS{
				Enabled: true,
				CA:      "-----BEGIN CERTIFICATE-----\nSECRET\n-----END CERTIFICATE-----",
			},
		},
		Readonly: true,
		Limit:    50,
	}
	sum := inst.ToSummary()
	b, err := json.Marshal(sum)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-kafka") || strings.Contains(string(b), "SECRET") {
		t.Fatalf("secret leaked in summary: %s", b)
	}
	if !sum.Connection.SASL.Enabled || sum.Connection.SASL.Username != "ro" {
		t.Fatalf("unexpected sasl summary: %+v", sum.Connection.SASL)
	}
	if !sum.Connection.TLS.HasCA {
		t.Fatal("expected has_ca")
	}
}

func loadTestConfig(t *testing.T, kafkaYAML string) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	kafkaPath := filepath.Join(dir, "kafka.yaml")
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	redisPath := filepath.Join(dir, "redis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	plugins := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(plugins, 0o755)
	_ = os.WriteFile(hostsPath, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(dbsPath, []byte("databases: []\n"), 0o600)
	_ = os.WriteFile(redisPath, []byte("redis: []\n"), 0o600)
	_ = os.WriteFile(kafkaPath, []byte(kafkaYAML), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: 18090
plugins:
  dir: "`+plugins+`"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
  redis: "`+redisPath+`"
  kafka: "`+kafkaPath+`"
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
	return cfg
}
