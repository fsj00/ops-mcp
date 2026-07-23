package model

// RedisTLS holds optional TLS / mTLS settings for a Redis connection.
//
// When Enabled is true, the client uses TLS. For mTLS, provide a client
// certificate via cert/private_key content or *_file paths.
// Content fields accept PEM text or standard base64 of the file bytes.
type RedisTLS struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	ServerName         string `yaml:"server_name" json:"server_name"`                   // SNI / verify name；缺省用 host
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" json:"insecure_skip_verify"` // 仅开发用

	// CA to verify the Redis server certificate (content or file).
	CA     string `yaml:"ca" json:"-"`
	CAFile string `yaml:"ca_file" json:"ca_file"`

	// Client certificate for mTLS (content or file).
	Cert     string `yaml:"cert" json:"-"`
	CertFile string `yaml:"cert_file" json:"cert_file"`

	// Client private key for mTLS (content or file).
	PrivateKey     string `yaml:"private_key" json:"-"`
	PrivateKeyFile string `yaml:"private_key_file" json:"-"`
}

// HasClientCert reports whether client certificate material is configured (mTLS).
func (t RedisTLS) HasClientCert() bool {
	hasCert := t.CertFile != "" || t.Cert != ""
	hasKey := t.PrivateKeyFile != "" || t.PrivateKey != ""
	return hasCert && hasKey
}

// HasCA reports whether a CA is configured for server verification.
func (t RedisTLS) HasCA() bool {
	return t.CAFile != "" || t.CA != ""
}

// RedisConnection holds Redis endpoint credentials.
// Logical DB index is NOT configured here — callers pass `db` per request (default 0).
type RedisConnection struct {
	Host     string   `yaml:"host" json:"host"`
	Port     int      `yaml:"port" json:"port"`
	Username string   `yaml:"username" json:"username"` // Redis 6+ ACL；可空
	Password string   `yaml:"password" json:"-"`        // 可空（无认证实例）
	TLS      RedisTLS `yaml:"tls" json:"tls"`
}

// RedisInstance is one redis.yaml entry.
type RedisInstance struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
	Connection  RedisConnection   `yaml:"connection" json:"connection"`
	Readonly    bool              `yaml:"readonly" json:"readonly"`
	// Limit caps SCAN / SLOWLOG / CLIENT LIST / ZRANGE sample sizes. Default 1000.
	Limit int `yaml:"limit" json:"limit"`
}

// RedisTLSSummary is a safe TLS view (no PEM / key material).
type RedisTLSSummary struct {
	Enabled            bool   `json:"enabled"`
	ServerName         string `json:"server_name,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	HasCA              bool   `json:"has_ca"`
	HasClientCert      bool   `json:"has_client_cert"`
}

// RedisConnectionSummary is a safe view of RedisConnection (no password / PEM).
type RedisConnectionSummary struct {
	Host     string          `json:"host"`
	Port     int             `json:"port"`
	Username string          `json:"username,omitempty"`
	TLS      RedisTLSSummary `json:"tls"`
}

// RedisSummary is a safe view for API/Agent (no password / PEM).
type RedisSummary struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Connection  RedisConnectionSummary `json:"connection"`
	Readonly    bool                   `json:"readonly"`
	Limit       int                    `json:"limit"`
}

// ToSummary strips password and certificate PEM material.
func (r RedisInstance) ToSummary() RedisSummary {
	tls := r.Connection.TLS
	return RedisSummary{
		Name:        r.Name,
		Description: r.Description,
		Labels:      r.Labels,
		Connection: RedisConnectionSummary{
			Host:     r.Connection.Host,
			Port:     r.Connection.Port,
			Username: r.Connection.Username,
			TLS: RedisTLSSummary{
				Enabled:            tls.Enabled,
				ServerName:         tls.ServerName,
				InsecureSkipVerify: tls.InsecureSkipVerify,
				HasCA:              tls.HasCA(),
				HasClientCert:      tls.HasClientCert(),
			},
		},
		Readonly: r.Readonly,
		Limit:    r.Limit,
	}
}

// RedisFile is the redis.yaml root.
type RedisFile struct {
	Redis []RedisInstance `yaml:"redis"`
}
