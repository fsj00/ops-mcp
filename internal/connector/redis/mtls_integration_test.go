package redis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const mtlsContainer = "ops-mcp-redis-mtls-test"
const mtlsPort = "26379"

// TestIntegrationRedisMTLS starts a Redis container with tls-auth-clients yes and
// verifies ops-mcp can dial with client certificates.
func TestIntegrationRedisMTLS(t *testing.T) {
	if os.Getenv("OPS_MCP_INTEGRATION") != "1" {
		t.Skip("set OPS_MCP_INTEGRATION=1 to run")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	dir := t.TempDir()
	caCert, caKey := mustGenCA(t)
	serverCert, serverKey := mustGenLeaf(t, caCert, caKey, "redis", []string{"127.0.0.1"}, false)
	clientCert, clientKey := mustGenLeaf(t, caCert, caKey, "ops-mcp-client", nil, true)

	caPath := filepath.Join(dir, "ca.crt")
	serverCertPath := filepath.Join(dir, "redis.crt")
	serverKeyPath := filepath.Join(dir, "redis.key")
	clientCertPath := filepath.Join(dir, "client.crt")
	clientKeyPath := filepath.Join(dir, "client.key")
	redisConf := filepath.Join(dir, "redis.conf")

	mustWrite(t, caPath, pemEncode("CERTIFICATE", caCert.Raw))
	mustWrite(t, serverCertPath, pemEncode("CERTIFICATE", serverCert.Raw))
	mustWrite(t, serverKeyPath, pemEncodeECKey(t, serverKey))
	mustWrite(t, clientCertPath, pemEncode("CERTIFICATE", clientCert.Raw))
	mustWrite(t, clientKeyPath, pemEncodeECKey(t, clientKey))
	mustWrite(t, redisConf, []byte(`
port 0
tls-port 6379
tls-cert-file /tls/redis.crt
tls-key-file /tls/redis.key
tls-ca-cert-file /tls/ca.crt
tls-auth-clients yes
protected-mode no
`))

	_ = exec.Command("docker", "rm", "-f", mtlsContainer).Run()
	run := exec.Command("docker", "run", "-d",
		"--name", mtlsContainer,
		"-p", mtlsPort+":6379",
		"-v", dir+":/tls:ro",
		"redis:7-alpine",
		"redis-server", "/tls/redis.conf",
	)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", mtlsContainer).Run()
	})

	// Wait until TLS port accepts connections with client cert.
	deadline := time.Now().Add(20 * time.Second)
	for {
		ping := exec.Command("docker", "exec", mtlsContainer,
			"redis-cli", "--tls",
			"--cert", "/tls/client.crt",
			"--key", "/tls/client.key",
			"--cacert", "/tls/ca.crt",
			"PING",
		)
		if out, err := ping.CombinedOutput(); err == nil && string(out) == "PONG\n" {
			break
		}
		if time.Now().After(deadline) {
			logs, _ := exec.Command("docker", "logs", mtlsContainer).CombinedOutput()
			t.Fatalf("redis mtls not ready; logs:\n%s", logs)
		}
		time.Sleep(300 * time.Millisecond)
	}

	cfgPath := writeMTLSTestConfig(t, dir, clientCertPath, clientKeyPath, caPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	c := New(cfg, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Without client cert must fail (mTLS required by server).
	_, err = c.Ping(ctx, PingRequest{Redis: "local-redis-tls-only"})
	if err == nil {
		t.Fatal("expected dial failure without client cert")
	}

	res, err := c.Ping(ctx, PingRequest{Redis: "local-redis-mtls"})
	if err != nil {
		t.Fatalf("mtls ping: %v", err)
	}
	if res.Result != "PONG" {
		t.Fatalf("result=%q", res.Result)
	}

	sum := cfg.ListRedisSummaries()
	var mtlsSum *model.RedisSummary
	for i := range sum {
		if sum[i].Name == "local-redis-mtls" {
			mtlsSum = &sum[i]
			break
		}
	}
	if mtlsSum == nil || !mtlsSum.Connection.TLS.Enabled || !mtlsSum.Connection.TLS.HasClientCert {
		t.Fatalf("summary missing mtls flags: %+v", sum)
	}
}

func writeMTLSTestConfig(t *testing.T, certDir, clientCert, clientKey, ca string) string {
	t.Helper()
	hosts := filepath.Join(certDir, "hosts.yaml")
	dbs := filepath.Join(certDir, "databases.yaml")
	rds := filepath.Join(certDir, "redis.yaml")
	cfgPath := filepath.Join(certDir, "ops-mcp.yaml")
	mustWrite(t, hosts, []byte("hosts: []\n"))
	mustWrite(t, dbs, []byte("databases: []\n"))

	redisFile := model.RedisFile{Redis: []model.RedisInstance{
		{
			Name:        "local-redis-mtls",
			Description: "mtls",
			Readonly:    true,
			Limit:       1000,
			Connection: model.RedisConnection{
				Host: "127.0.0.1",
				Port: 26379,
				TLS: model.RedisTLS{
					Enabled:        true,
					ServerName:     "localhost",
					CAFile:         ca,
					CertFile:       clientCert,
					PrivateKeyFile: clientKey,
				},
			},
		},
		{
			Name:        "local-redis-tls-only",
			Description: "tls without client cert",
			Readonly:    true,
			Limit:       1000,
			Connection: model.RedisConnection{
				Host: "127.0.0.1",
				Port: 26379,
				TLS: model.RedisTLS{
					Enabled:    true,
					ServerName: "localhost",
					CAFile:     ca,
				},
			},
		},
	}}
	b, err := yaml.Marshal(&redisFile)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, rds, b)
	mustWrite(t, cfgPath, []byte(fmt.Sprintf(`
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: %q
  databases: %q
  redis: %q
defaults:
  plugin_timeout: 10s
log:
  level: error
  encoding: console
`, hosts, dbs, rds)))
	return cfgPath
}
