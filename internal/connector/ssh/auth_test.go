package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsj00/ops-mcp/internal/model"
	gossh "golang.org/x/crypto/ssh"
)

func TestBuildAuthPassword(t *testing.T) {
	methods, err := buildAuth(model.Host{
		Auth: model.HostAuth{Type: "password", Username: "root", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("got %d auth methods", len(methods))
	}
}

func TestBuildAuthPasswordMissing(t *testing.T) {
	_, err := buildAuth(model.Host{
		Auth: model.HostAuth{Type: "password", Username: "root"},
	})
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}

func TestBuildAuthPrivateKeyInline(t *testing.T) {
	pemBytes := mustGenEd25519PEM(t)
	methods, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:       "private_key",
			Username:   "ubuntu",
			PrivateKey: string(pemBytes),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("got %d auth methods", len(methods))
	}
}

func TestBuildAuthPrivateKeyFile(t *testing.T) {
	pemBytes := mustGenEd25519PEM(t)
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	methods, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:           "private_key",
			Username:       "ubuntu",
			PrivateKeyFile: path,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("got %d auth methods", len(methods))
	}
}

func TestBuildAuthPrivateKeyBase64(t *testing.T) {
	pemBytes := mustGenEd25519PEM(t)
	methods, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:       "private_key",
			Username:   "ubuntu",
			PrivateKey: base64.StdEncoding.EncodeToString(pemBytes),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("got %d auth methods", len(methods))
	}
}

func TestBuildAuthPrivateKeyRejectBoth(t *testing.T) {
	pemBytes := mustGenEd25519PEM(t)
	_, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:           "private_key",
			Username:       "ubuntu",
			PrivateKey:     string(pemBytes),
			PrivateKeyFile: "/tmp/key.pem",
		},
	})
	if err == nil {
		t.Fatal("expected error when both private_key and private_key_file set")
	}
}

func TestBuildAuthPrivateKeyMissing(t *testing.T) {
	_, err := buildAuth(model.Host{
		Auth: model.HostAuth{Type: "private_key", Username: "ubuntu"},
	})
	if err == nil {
		t.Fatal("expected error when neither private_key nor private_key_file set")
	}
}

func TestBuildAuthPrivateKeyFileMissing(t *testing.T) {
	_, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:           "private_key",
			Username:       "ubuntu",
			PrivateKeyFile: filepath.Join(t.TempDir(), "missing.pem"),
		},
	})
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestBuildAuthPrivateKeyTrimmedPEM(t *testing.T) {
	keyPEM := mustGenEd25519PEM(t)
	methods, err := buildAuth(model.Host{
		Auth: model.HostAuth{
			Type:       "private_key",
			Username:   "ubuntu",
			PrivateKey: "  " + string(keyPEM) + "  ",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("got %d auth methods", len(methods))
	}
	if _, err := gossh.ParsePrivateKey(keyPEM); err != nil {
		t.Fatalf("generated key should parse: %v", err)
	}
}

func mustGenEd25519PEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(b)
}
