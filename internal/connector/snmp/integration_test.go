//go:build integration

package snmp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
)

// Requires local snmpsim: make snmp-up
//
//	OPS_MCP_INTEGRATION=1 go test -tags=integration ./internal/connector/snmp/ -count=1 -run Local
func TestLocalSNMPSimulator(t *testing.T) {
	if os.Getenv("OPS_MCP_INTEGRATION") != "1" {
		t.Skip("set OPS_MCP_INTEGRATION=1")
	}
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	snmp := filepath.Join(dir, "snmp.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(hosts, "hosts: []\n")
	write(dbs, "databases: []\n")
	write(snmp, `
credentials:
  - name: local-ro-v2c
    version: 2c
    community: "public"
  - name: local-ro-v3
    version: 3
    security_level: authPriv
    username: readonly
    auth_protocol: SHA
    auth_password: "authpass123"
    priv_protocol: AES
    priv_password: "privpass123"
devices:
  - name: local-snmp
    address: { host: 127.0.0.1, port: 1161 }
    credential: local-ro-v2c
  - name: local-snmp-v3
    address: { host: 127.0.0.1, port: 1161 }
    credential: local-ro-v3
    context: public
`)
	write(cfgPath, `
server: { port: 20267 }
plugins: { dir: "./plugins" }
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
  snmp: "`+snmp+`"
`)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	c := New(cfg, nil)
	ctx := context.Background()

	res, err := c.Get(ctx, GetRequest{Device: "local-snmp", OIDs: []string{"1.3.6.1.2.1.1.5.0"}})
	if err != nil {
		t.Fatalf("v2c get: %v (is snmpsim up? make snmp-up)", err)
	}
	if res.Count != 1 || res.Vars[0].Value != "sw-local-sim-01" {
		t.Fatalf("v2c res=%+v", res)
	}

	res, err = c.Get(ctx, GetRequest{Device: "local-snmp-v3", OIDs: []string{"1.3.6.1.2.1.1.5.0"}})
	if err != nil {
		t.Fatalf("v3 get: %v", err)
	}
	if res.Count != 1 || res.Vars[0].Value != "sw-local-sim-01" {
		t.Fatalf("v3 res=%+v", res)
	}
}
