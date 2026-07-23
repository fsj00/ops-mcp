package model

// SNMPAuth holds SNMPv2c or SNMPv3 authentication parameters (no name).
// Used both as credentials[] entries (via SNMPCredential) and device inline auth.
type SNMPAuth struct {
	Version       string `yaml:"version" json:"version"`                         // "2c" | "3"
	Community     string `yaml:"community" json:"-"`                             // v2c
	SecurityLevel string `yaml:"security_level" json:"security_level,omitempty"` // noAuthNoPriv | authNoPriv | authPriv
	Username      string `yaml:"username" json:"username,omitempty"`
	AuthProtocol  string `yaml:"auth_protocol" json:"auth_protocol,omitempty"` // MD5 | SHA | SHA224 | SHA256 | SHA384 | SHA512
	AuthPassword  string `yaml:"auth_password" json:"-"`
	PrivProtocol  string `yaml:"priv_protocol" json:"priv_protocol,omitempty"` // DES | AES | AES192 | AES256
	PrivPassword  string `yaml:"priv_password" json:"-"`
}

// SNMPAuthSummary is a safe view of SNMPAuth (no secrets).
type SNMPAuthSummary struct {
	Version       string `json:"version"`
	SecurityLevel string `json:"security_level,omitempty"`
	Username      string `json:"username,omitempty"`
	AuthProtocol  string `json:"auth_protocol,omitempty"`
	PrivProtocol  string `json:"priv_protocol,omitempty"`
	HasCommunity  bool   `json:"has_community,omitempty"`
	HasAuthPass   bool   `json:"has_auth_password,omitempty"`
	HasPrivPass   bool   `json:"has_priv_password,omitempty"`
}

// ToSummary strips secrets from SNMPAuth.
func (a SNMPAuth) ToSummary() SNMPAuthSummary {
	return SNMPAuthSummary{
		Version:       a.Version,
		SecurityLevel: a.SecurityLevel,
		Username:      a.Username,
		AuthProtocol:  a.AuthProtocol,
		PrivProtocol:  a.PrivProtocol,
		HasCommunity:  a.Community != "",
		HasAuthPass:   a.AuthPassword != "",
		HasPrivPass:   a.PrivPassword != "",
	}
}

// SNMPCredential is a named auth profile in snmp.yaml credentials[].
type SNMPCredential struct {
	Name string `yaml:"name" json:"name"`
	SNMPAuth `yaml:",inline"`
}

// SNMPAddress is the UDP endpoint for a device.
type SNMPAddress struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// SNMPDevice is one snmp.yaml devices[] entry.
// Exactly one of Credential (profile name) or Auth (inline) must be set.
type SNMPDevice struct {
	Name           string            `yaml:"name" json:"name"`
	Description    string            `yaml:"description" json:"description"`
	Labels         map[string]string `yaml:"labels" json:"labels,omitempty"`
	Address        SNMPAddress       `yaml:"address" json:"address"`
	Credential     string            `yaml:"credential" json:"credential,omitempty"`
	Auth           *SNMPAuth         `yaml:"auth" json:"-"`
	// Context is SNMPv3 contextName (also used by snmpsim to select data file / community).
	Context        string            `yaml:"context" json:"context,omitempty"`
	Timeout        string            `yaml:"timeout" json:"timeout,omitempty"`                   // Go duration, e.g. 5s
	Retries        *int              `yaml:"retries" json:"retries,omitempty"`
	MaxRepetitions *int              `yaml:"max_repetitions" json:"max_repetitions,omitempty"`
	WalkMaxOIDs    *int              `yaml:"walk_max_oids" json:"walk_max_oids,omitempty"`
}

// SNMPDeviceSummary is a safe view for API/Agent (no secrets).
type SNMPDeviceSummary struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Labels         map[string]string `json:"labels,omitempty"`
	Address        SNMPAddress       `json:"address"`
	AuthMode       string            `json:"auth_mode"` // "credential" | "inline"
	Credential     string            `json:"credential,omitempty"`
	Auth           *SNMPAuthSummary  `json:"auth,omitempty"` // version/username only when inline
	Context        string            `json:"context,omitempty"`
	Timeout        string            `json:"timeout,omitempty"`
	Retries        *int              `json:"retries,omitempty"`
	MaxRepetitions *int              `json:"max_repetitions,omitempty"`
	WalkMaxOIDs    *int              `json:"walk_max_oids,omitempty"`
}

// ToSummary strips secrets from SNMPDevice.
func (d SNMPDevice) ToSummary() SNMPDeviceSummary {
	s := SNMPDeviceSummary{
		Name:           d.Name,
		Description:    d.Description,
		Labels:         d.Labels,
		Address:        d.Address,
		Context:        d.Context,
		Timeout:        d.Timeout,
		Retries:        d.Retries,
		MaxRepetitions: d.MaxRepetitions,
		WalkMaxOIDs:    d.WalkMaxOIDs,
	}
	if d.Credential != "" {
		s.AuthMode = "credential"
		s.Credential = d.Credential
	} else if d.Auth != nil {
		s.AuthMode = "inline"
		sum := d.Auth.ToSummary()
		s.Auth = &sum
	}
	return s
}

// SNMPDefaults holds file-level defaults for devices.
type SNMPDefaults struct {
	Timeout        string `yaml:"timeout" json:"timeout"`
	Retries        int    `yaml:"retries" json:"retries"`
	MaxRepetitions int    `yaml:"max_repetitions" json:"max_repetitions"`
	WalkMaxOIDs    int    `yaml:"walk_max_oids" json:"walk_max_oids"`
}

// SNMPFile is the snmp.yaml root.
type SNMPFile struct {
	Credentials []SNMPCredential `yaml:"credentials"`
	Devices     []SNMPDevice     `yaml:"devices"`
	Defaults    SNMPDefaults     `yaml:"defaults"`
}
