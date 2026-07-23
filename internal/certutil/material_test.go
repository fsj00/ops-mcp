package certutil

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveMaterialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")
	want := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveMaterial("", path, "private_key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMaterialPEM(t *testing.T) {
	pem := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"
	got, err := ResolveMaterial(pem, "", "ca")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != pem {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMaterialBase64(t *testing.T) {
	raw := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----")
	b64 := base64.StdEncoding.EncodeToString(raw)
	got, err := ResolveMaterial(b64, "", "ca")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMaterialRejectBoth(t *testing.T) {
	_, err := ResolveMaterial("-----BEGIN X-----", "/tmp/x.pem", "ca")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveMaterialOptionalEmpty(t *testing.T) {
	got, err := ResolveMaterial("", "", "ca")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveMaterialInvalidBase64(t *testing.T) {
	_, err := ResolveMaterial("not-valid-base64!!!", "", "ca")
	if err == nil {
		t.Fatal("expected error")
	}
}
