package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMinimalOpsMCP(t *testing.T, dir, snmpYAML string) string {
	t.Helper()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	snmpPath := filepath.Join(dir, "snmp.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	mustWrite(t, hostsPath, "hosts: []\n")
	mustWrite(t, dbsPath, "databases: []\n")
	mustWrite(t, snmpPath, snmpYAML)
	mustWrite(t, cfgPath, `
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
  snmp: "`+snmpPath+`"
`)
	return cfgPath
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSNMPCredentialAndInline(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalOpsMCP(t, dir, `
credentials:
  - name: shared-v2c
    version: 2c
    community: "secret-ro"
devices:
  - name: sw-ref
    labels: { site: dc1, role: core }
    address: { host: 10.0.0.1 }
    credential: shared-v2c
  - name: sw-inline
    labels: { site: lab }
    address: { host: 10.0.0.2, port: 1161 }
    auth:
      version: 2c
      community: "lab-secret"
defaults:
  timeout: 3s
  walk_max_oids: 500
`)
	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	d, err := m.GetSNMPDevice("sw-ref")
	if err != nil {
		t.Fatal(err)
	}
	if d.Address.Port != 161 {
		t.Fatalf("default port=%d", d.Address.Port)
	}
	auth, err := m.ResolveSNMPAuth("sw-ref")
	if err != nil {
		t.Fatal(err)
	}
	if auth.Community != "secret-ro" || auth.Version != "2c" {
		t.Fatalf("resolved auth=%+v", auth)
	}

	auth2, err := m.ResolveSNMPAuth("sw-inline")
	if err != nil {
		t.Fatal(err)
	}
	if auth2.Community != "lab-secret" {
		t.Fatalf("inline auth=%+v", auth2)
	}

	sum := m.ListSNMPDeviceSummaries(SNMPDeviceListOptions{})
	if len(sum) != 2 {
		t.Fatalf("summaries=%+v", sum)
	}
	// sorted by name: sw-inline, sw-ref
	if sum[0].Name != "sw-inline" || sum[0].AuthMode != "inline" {
		t.Fatalf("sum[0]=%+v", sum[0])
	}
	if sum[0].Auth == nil || sum[0].Auth.HasCommunity != true {
		t.Fatalf("inline summary auth=%+v", sum[0].Auth)
	}
	// secrets must not appear in JSON-facing summary strings via Community field (json:"-")
	if sum[1].AuthMode != "credential" || sum[1].Credential != "shared-v2c" {
		t.Fatalf("sum[1]=%+v", sum[1])
	}

	filtered := m.ListSNMPDeviceSummaries(SNMPDeviceListOptions{
		Labels: map[string]string{"site": "dc1"},
	})
	if len(filtered) != 1 || filtered[0].Name != "sw-ref" {
		t.Fatalf("label filter=%+v", filtered)
	}

	paged := m.ListSNMPDeviceSummaries(SNMPDeviceListOptions{Limit: 1, Offset: 1})
	if len(paged) != 1 || paged[0].Name != "sw-ref" {
		t.Fatalf("page=%+v", paged)
	}

	defs := m.SNMPDefaults()
	if defs.Timeout != "3s" || defs.WalkMaxOIDs != 500 || defs.Retries != DefaultSNMPRetries {
		t.Fatalf("defaults=%+v", defs)
	}
}

func TestLoadSNMPRejectsBothAuthModes(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalOpsMCP(t, dir, `
credentials:
  - name: shared
    version: 2c
    community: "x"
devices:
  - name: bad
    address: { host: 1.2.3.4 }
    credential: shared
    auth:
      version: 2c
      community: "y"
`)
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected load error for both credential and auth")
	}
}

func TestLoadSNMPRejectsNeitherAuthMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalOpsMCP(t, dir, `
devices:
  - name: bad
    address: { host: 1.2.3.4 }
`)
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected load error for missing credential/auth")
	}
}

func TestLoadSNMPRejectsUnknownCredential(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalOpsMCP(t, dir, `
devices:
  - name: bad
    address: { host: 1.2.3.4 }
    credential: missing
`)
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected load error for unknown credential")
	}
}

func TestLoadSNMPMissingFileEmpty(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")
	dbsPath := filepath.Join(dir, "databases.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	mustWrite(t, hostsPath, "hosts: []\n")
	mustWrite(t, dbsPath, "databases: []\n")
	mustWrite(t, cfgPath, `
server:
  port: 20267
plugins:
  dir: "./plugins"
config:
  hosts: "`+hostsPath+`"
  databases: "`+dbsPath+`"
  snmp: "`+filepath.Join(dir, "no-snmp.yaml")+`"
`)
	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.ListSNMPDeviceSummaries(SNMPDeviceListOptions{Limit: 10})) != 0 {
		t.Fatal("expected empty snmp devices")
	}
}

func TestLoadSNMPv3Validation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeMinimalOpsMCP(t, dir, `
devices:
  - name: v3dev
    address: { host: 1.2.3.4 }
    auth:
      version: 3
      security_level: authPriv
      username: u
      auth_protocol: SHA
      auth_password: "ap"
      priv_protocol: AES
      priv_password: "pp"
`)
	m, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	auth, err := m.ResolveSNMPAuth("v3dev")
	if err != nil {
		t.Fatal(err)
	}
	if auth.Username != "u" || auth.AuthPassword != "ap" {
		t.Fatalf("auth=%+v", auth)
	}
}
