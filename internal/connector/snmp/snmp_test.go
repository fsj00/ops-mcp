package snmp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/gosnmp/gosnmp"
)

type fakeClient struct {
	getFn      func(oids []string) (*gosnmp.SnmpPacket, error)
	walkFn     func(root string, fn gosnmp.WalkFunc) error
	bulkWalkFn func(root string, fn gosnmp.WalkFunc) error
	closed     bool
}

func (f *fakeClient) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	if f.getFn != nil {
		return f.getFn(oids)
	}
	return &gosnmp.SnmpPacket{}, nil
}

func (f *fakeClient) Walk(root string, fn gosnmp.WalkFunc) error {
	if f.walkFn != nil {
		return f.walkFn(root, fn)
	}
	return nil
}

func (f *fakeClient) BulkWalk(root string, fn gosnmp.WalkFunc) error {
	if f.bulkWalkFn != nil {
		return f.bulkWalkFn(root, fn)
	}
	return nil
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

func loadSNMPTestConfig(t *testing.T, snmpYAML string) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	snmpPath := filepath.Join(dir, "snmp.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(hostsPath, "hosts: []\n")
	write(dbsPath, "databases: []\n")
	write(snmpPath, snmpYAML)
	write(cfgPath, `
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
  snmp: "`+snmpPath+`"
`)
	m, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return m
}

func TestGetValidation(t *testing.T) {
	cfg := loadSNMPTestConfig(t, `
devices:
  - name: sw1
    address: { host: 127.0.0.1 }
    auth: { version: 2c, community: "public" }
`)
	c := New(cfg, nil)
	_, err := c.Get(context.Background(), GetRequest{})
	if err == nil || !strings.Contains(err.Error(), "device") {
		t.Fatalf("err=%v", err)
	}
	_, err = c.Get(context.Background(), GetRequest{Device: "sw1"})
	if err == nil || !strings.Contains(err.Error(), "oids") {
		t.Fatalf("err=%v", err)
	}
	_, err = c.Get(context.Background(), GetRequest{Device: "missing", OIDs: []string{"1.3.6"}})
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "public") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func TestGetSuccess(t *testing.T) {
	cfg := loadSNMPTestConfig(t, `
devices:
  - name: sw1
    address: { host: 10.0.0.1 }
    auth: { version: 2c, community: "secret-community" }
`)
	c := New(cfg, nil)
	fake := &fakeClient{
		getFn: func(oids []string) (*gosnmp.SnmpPacket, error) {
			if len(oids) != 1 || oids[0] != "1.3.6.1.2.1.1.1.0" {
				t.Fatalf("oids=%v", oids)
			}
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{{
					Name:  "1.3.6.1.2.1.1.1.0",
					Type:  gosnmp.OctetString,
					Value: []byte("switch"),
				}},
			}, nil
		},
	}
	c.dial = func(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (snmpClient, error) {
		if auth.Community != "secret-community" {
			t.Fatalf("auth=%+v", auth)
		}
		return fake, nil
	}
	res, err := c.Get(context.Background(), GetRequest{
		Device: "sw1",
		OIDs:   []string{"1.3.6.1.2.1.1.1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || res.Vars[0].Value != "switch" || !fake.closed {
		t.Fatalf("res=%+v closed=%v", res, fake.closed)
	}
}

func TestWalkTruncates(t *testing.T) {
	cfg := loadSNMPTestConfig(t, `
devices:
  - name: sw1
    address: { host: 10.0.0.1 }
    auth: { version: 2c, community: "x" }
    walk_max_oids: 2
`)
	c := New(cfg, nil)
	c.dial = func(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (snmpClient, error) {
		return &fakeClient{
			walkFn: func(root string, fn gosnmp.WalkFunc) error {
				for i := 0; i < 5; i++ {
					if err := fn(gosnmp.SnmpPDU{
						Name:  "1.3.6." + string(rune('0'+i)),
						Type:  gosnmp.Integer,
						Value: i,
					}); err != nil {
						return err
					}
				}
				return nil
			},
		}, nil
	}
	res, err := c.Walk(context.Background(), WalkRequest{Device: "sw1", OID: "1.3.6"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.Count != 2 {
		t.Fatalf("res=%+v", res)
	}
}

func TestConnectFailureNoSecretLeak(t *testing.T) {
	cfg := loadSNMPTestConfig(t, `
devices:
  - name: sw1
    address: { host: 127.0.0.1 }
    auth: { version: 2c, community: "super-secret" }
`)
	c := New(cfg, nil)
	c.dial = func(ctx context.Context, device model.SNMPDevice, auth model.SNMPAuth, settings deviceSettings) (snmpClient, error) {
		return nil, errors.New("dial refused")
	}
	_, err := c.Get(context.Background(), GetRequest{Device: "sw1", OIDs: []string{"1.3.6"}})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrConnectorError {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func TestBuildGoSNMPv3(t *testing.T) {
	g, err := buildGoSNMP(context.Background(), model.SNMPDevice{
		Address: model.SNMPAddress{Host: "1.2.3.4", Port: 161},
	}, model.SNMPAuth{
		Version:       "3",
		SecurityLevel: "authPriv",
		Username:      "u",
		AuthProtocol:  "SHA",
		AuthPassword:  "ap",
		PrivProtocol:  "AES",
		PrivPassword:  "pp",
	}, deviceSettings{Timeout: 0, Retries: 1, MaxRepetitions: 25})
	if err != nil {
		t.Fatal(err)
	}
	if g.Version != gosnmp.Version3 || g.MsgFlags != gosnmp.AuthPriv {
		t.Fatalf("g=%+v", g)
	}
	usm := g.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if usm.UserName != "u" || usm.AuthenticationProtocol != gosnmp.SHA {
		t.Fatalf("usm=%+v", usm)
	}
}

func TestToSummaryStripsSecrets(t *testing.T) {
	d := model.SNMPDevice{
		Name: "sw1",
		Auth: &model.SNMPAuth{Version: "2c", Community: "secret"},
	}
	s := d.ToSummary()
	if s.AuthMode != "inline" || s.Auth == nil || !s.Auth.HasCommunity {
		t.Fatalf("summary=%+v", s)
	}
}
