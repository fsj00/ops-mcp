package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/config"
	snmpconn "github.com/fsj00/ops-mcp/internal/connector/snmp"
)

func TestSNMPDevicesListBridge(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yaml")
	dbs := filepath.Join(dir, "databases.yaml")
	snmp := filepath.Join(dir, "snmp.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(dbs, []byte("databases: []\n"), 0o600)
	_ = os.WriteFile(snmp, []byte(`
devices:
  - name: sw1
    labels: { site: dc1 }
    address: { host: 10.0.0.1 }
    auth: { version: 2c, community: "x" }
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server: { port: 20267 }
plugins: { dir: "./plugins" }
config:
  hosts: "`+hosts+`"
  databases: "`+dbs+`"
  snmp: "`+snmp+`"
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	rt := New(Dependencies{
		Cfg:  cfg,
		SNMP: snmpconn.New(cfg, nil),
	})
	res, err := rt.Execute(context.Background(), `
function execute(ctx) {
  var devices = ctx.snmp_devices.list({ labels: { site: "dc1" }, limit: 10 });
  return { count: devices.length, name: devices[0].name, mode: devices[0].auth_mode };
}
`, map[string]interface{}{}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("res=%T", res)
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
	if m["name"] != "sw1" || m["mode"] != "inline" {
		t.Fatalf("m=%+v", m)
	}
}
