package model

// KafkaTLS holds optional TLS / mTLS settings for a Kafka connection.
type KafkaTLS struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	ServerName         string `yaml:"server_name" json:"server_name"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`

	CA     string `yaml:"ca" json:"-"`
	CAFile string `yaml:"ca_file" json:"ca_file"`

	Cert     string `yaml:"cert" json:"-"`
	CertFile string `yaml:"cert_file" json:"cert_file"`

	PrivateKey     string `yaml:"private_key" json:"-"`
	PrivateKeyFile string `yaml:"private_key_file" json:"-"`
}

func (t KafkaTLS) HasClientCert() bool {
	hasCert := t.CertFile != "" || t.Cert != ""
	hasKey := t.PrivateKeyFile != "" || t.PrivateKey != ""
	return hasCert && hasKey
}

func (t KafkaTLS) HasCA() bool {
	return t.CAFile != "" || t.CA != ""
}

// KafkaSASL holds optional SASL authentication.
// Mechanism: plain | scram-sha-256 | scram-sha-512
type KafkaSASL struct {
	Mechanism string `yaml:"mechanism" json:"mechanism"`
	Username  string `yaml:"username" json:"username"`
	Password  string `yaml:"password" json:"-"`
}

// KafkaConnection holds Kafka bootstrap brokers and optional auth/TLS.
type KafkaConnection struct {
	Brokers []string  `yaml:"brokers" json:"brokers"`
	SASL    KafkaSASL `yaml:"sasl" json:"sasl"`
	TLS     KafkaTLS  `yaml:"tls" json:"tls"`
}

// KafkaInstance is one kafka.yaml entry.
type KafkaInstance struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
	Connection  KafkaConnection   `yaml:"connection" json:"connection"`
	Readonly    bool              `yaml:"readonly" json:"readonly"`
	// Limit caps topic/group list sizes. Default 1000.
	Limit int `yaml:"limit" json:"limit"`
}

// KafkaTLSSummary is a safe TLS view (no PEM / key material).
type KafkaTLSSummary struct {
	Enabled            bool   `json:"enabled"`
	ServerName         string `json:"server_name,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	HasCA              bool   `json:"has_ca"`
	HasClientCert      bool   `json:"has_client_cert"`
}

// KafkaSASLSummary is a safe SASL view (no password).
type KafkaSASLSummary struct {
	Mechanism string `json:"mechanism,omitempty"`
	Username  string `json:"username,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// KafkaConnectionSummary is a safe view of KafkaConnection.
type KafkaConnectionSummary struct {
	Brokers []string         `json:"brokers"`
	SASL    KafkaSASLSummary `json:"sasl"`
	TLS     KafkaTLSSummary  `json:"tls"`
}

// KafkaSummary is a safe view for API/Agent (no password / PEM).
type KafkaSummary struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Connection  KafkaConnectionSummary `json:"connection"`
	Readonly    bool                   `json:"readonly"`
	Limit       int                    `json:"limit"`
}

// ToSummary strips password and certificate PEM material.
func (k KafkaInstance) ToSummary() KafkaSummary {
	tls := k.Connection.TLS
	sasl := k.Connection.SASL
	return KafkaSummary{
		Name:        k.Name,
		Description: k.Description,
		Labels:      k.Labels,
		Connection: KafkaConnectionSummary{
			Brokers: append([]string(nil), k.Connection.Brokers...),
			SASL: KafkaSASLSummary{
				Mechanism: sasl.Mechanism,
				Username:  sasl.Username,
				Enabled:   sasl.Username != "" || sasl.Password != "" || sasl.Mechanism != "",
			},
			TLS: KafkaTLSSummary{
				Enabled:            tls.Enabled,
				ServerName:         tls.ServerName,
				InsecureSkipVerify: tls.InsecureSkipVerify,
				HasCA:              tls.HasCA(),
				HasClientCert:      tls.HasClientCert(),
			},
		},
		Readonly: k.Readonly,
		Limit:    k.Limit,
	}
}

// KafkaFile is the kafka.yaml root.
type KafkaFile struct {
	Kafka []KafkaInstance `yaml:"kafka"`
}
