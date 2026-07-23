package redis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/model"
)

func TestBuildTLSConfigDisabled(t *testing.T) {
	cfg, err := buildTLSConfig("127.0.0.1", model.RedisTLS{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("expected nil when disabled")
	}
}

func TestBuildTLSConfigMTLSFromFiles(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := mustGenCA(t)
	clientCert, clientKey := mustGenLeaf(t, caCert, caKey, "client", nil, true)
	serverCert, _ := mustGenLeaf(t, caCert, caKey, "redis", []string{"127.0.0.1"}, false)
	_ = serverCert

	caPath := filepath.Join(dir, "ca.crt")
	certPath := filepath.Join(dir, "client.crt")
	keyPath := filepath.Join(dir, "client.key")
	mustWrite(t, caPath, pemEncode("CERTIFICATE", caCert.Raw))
	mustWrite(t, certPath, pemEncode("CERTIFICATE", clientCert.Raw))
	mustWrite(t, keyPath, pemEncodeECKey(t, clientKey))

	tlsCfg, err := buildTLSConfig("127.0.0.1", model.RedisTLS{
		Enabled:        true,
		CAFile:         caPath,
		CertFile:       certPath,
		PrivateKeyFile: keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tlsCfg == nil || tlsCfg.RootCAs == nil || len(tlsCfg.Certificates) != 1 {
		t.Fatalf("unexpected tls config: %+v", tlsCfg)
	}
	if tlsCfg.ServerName != "127.0.0.1" {
		t.Fatalf("server_name=%q", tlsCfg.ServerName)
	}
}

func TestBuildTLSConfigInlineAndRejectBoth(t *testing.T) {
	caCert, caKey := mustGenCA(t)
	clientCert, clientKey := mustGenLeaf(t, caCert, caKey, "client", nil, true)
	caPEM := string(pemEncode("CERTIFICATE", caCert.Raw))
	certPEM := string(pemEncode("CERTIFICATE", clientCert.Raw))
	keyPEM := string(pemEncodeECKey(t, clientKey))

	tlsCfg, err := buildTLSConfig("redis.local", model.RedisTLS{
		Enabled:    true,
		ServerName: "redis.local",
		CA:         caPEM,
		Cert:       certPEM,
		PrivateKey: keyPEM,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tlsCfg.ServerName != "redis.local" || len(tlsCfg.Certificates) != 1 {
		t.Fatalf("bad inline tls: %+v", tlsCfg)
	}

	_, err = buildTLSConfig("h", model.RedisTLS{
		Enabled: true,
		CAFile:  "/tmp/ca.crt",
		CA:      caPEM,
	})
	if err == nil {
		t.Fatal("expected error when both ca and ca_file set")
	}
}

func TestBuildTLSConfigBase64(t *testing.T) {
	caCert, caKey := mustGenCA(t)
	clientCert, clientKey := mustGenLeaf(t, caCert, caKey, "client", nil, true)
	caPEM := pemEncode("CERTIFICATE", caCert.Raw)
	certPEM := pemEncode("CERTIFICATE", clientCert.Raw)
	keyPEM := pemEncodeECKey(t, clientKey)

	tlsCfg, err := buildTLSConfig("redis.local", model.RedisTLS{
		Enabled:    true,
		ServerName: "redis.local",
		CA:         base64.StdEncoding.EncodeToString(caPEM),
		Cert:       base64.StdEncoding.EncodeToString(certPEM),
		PrivateKey: base64.StdEncoding.EncodeToString(keyPEM),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tlsCfg.Certificates) != 1 || tlsCfg.RootCAs == nil {
		t.Fatalf("bad base64 tls: %+v", tlsCfg)
	}
}

func TestBuildTLSConfigIncompleteClientPair(t *testing.T) {
	_, err := buildTLSConfig("h", model.RedisTLS{
		Enabled:  true,
		CertFile: "/tmp/client.crt",
	})
	if err == nil {
		t.Fatal("expected error for missing private_key")
	}
}

func mustGenCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ops-mcp-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func mustGenLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, ips []string, client bool) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	if client {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		for _, ip := range ips {
			tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP(ip))
		}
		tmpl.DNSNames = []string{"localhost"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func pemEncode(typ string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
}

func pemEncodeECKey(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
