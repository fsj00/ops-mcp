package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/fsj00/ops-mcp/internal/certutil"
	"github.com/fsj00/ops-mcp/internal/model"
)

// buildTLSConfig builds a *tls.Config from redis.yaml TLS settings.
// Returns nil when TLS is disabled.
func buildTLSConfig(host string, cfg model.RedisTLS) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // explicit opt-in for local/dev
	}
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	} else if host != "" {
		tlsCfg.ServerName = host
	}

	if cfg.HasCA() {
		pem, err := certutil.ResolveMaterial(cfg.CA, cfg.CAFile, "redis tls ca")
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("redis tls: failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
	}

	hasCert := cfg.CertFile != "" || cfg.Cert != ""
	hasKey := cfg.PrivateKeyFile != "" || cfg.PrivateKey != ""
	if hasCert || hasKey {
		certPEM, err := certutil.ResolveMaterial(cfg.Cert, cfg.CertFile, "redis tls client cert")
		if err != nil {
			return nil, err
		}
		keyPEM, err := certutil.ResolveMaterial(cfg.PrivateKey, cfg.PrivateKeyFile, "redis tls private_key")
		if err != nil {
			return nil, err
		}
		if len(certPEM) == 0 || len(keyPEM) == 0 {
			return nil, fmt.Errorf("redis tls: mTLS requires both client cert and private_key")
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("redis tls: load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
