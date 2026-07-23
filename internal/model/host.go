package model

// HostAddress is the SSH endpoint.
type HostAddress struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// HostAuth is SSH authentication material.
type HostAuth struct {
	Type           string `yaml:"type" json:"type"`
	Username       string `yaml:"username" json:"username"`
	Password       string `yaml:"password" json:"-"`
	PrivateKey     string `yaml:"private_key" json:"-"`
	PrivateKeyFile string `yaml:"private_key_file" json:"-"`
}

// Host is one entry from hosts.yaml.
type Host struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
	Address     HostAddress       `yaml:"address" json:"address"`
	Auth        HostAuth          `yaml:"auth" json:"auth"`
}

// HostSummary is a safe view for Agent/Tool responses (no secrets).
type HostSummary struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels,omitempty"`
	Address     HostAddress       `json:"address"`
	AuthType    string            `json:"auth_type"`
	Username    string            `json:"username"`
}

// ToSummary strips passwords and private keys.
func (h Host) ToSummary() HostSummary {
	return HostSummary{
		Name:        h.Name,
		Description: h.Description,
		Labels:      h.Labels,
		Address:     h.Address,
		AuthType:    h.Auth.Type,
		Username:    h.Auth.Username,
	}
}

// HostsFile is the root of hosts.yaml.
type HostsFile struct {
	Hosts []Host `yaml:"hosts"`
}
