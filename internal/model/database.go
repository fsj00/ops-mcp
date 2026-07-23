package model

// DBConnection holds database connection parameters.
type DBConnection struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"-"`
	Database string `yaml:"database" json:"database"`
	SSLMode  string `yaml:"sslmode" json:"sslmode"`
}

// Database is one entry from databases.yaml.
type Database struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
	Type        string            `yaml:"type" json:"type"`
	Connection  DBConnection      `yaml:"connection" json:"connection"`
	Readonly    bool              `yaml:"readonly" json:"readonly"`
	// Limit caps SELECT result rows. Zero/negative means default (1000).
	Limit int `yaml:"limit" json:"limit"`
}

// DBConnectionSummary is a safe view of DBConnection (no password).
type DBConnectionSummary struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Database string `json:"database"`
	SSLMode  string `json:"sslmode,omitempty"`
}

// DatabaseSummary is a safe view for API/Agent (no password).
type DatabaseSummary struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Labels      map[string]string   `json:"labels,omitempty"`
	Type        string              `json:"type"`
	Connection  DBConnectionSummary `json:"connection"`
	Readonly    bool                `json:"readonly"`
	Limit       int                 `json:"limit"`
}

// ToSummary strips password.
func (d Database) ToSummary() DatabaseSummary {
	return DatabaseSummary{
		Name:        d.Name,
		Description: d.Description,
		Labels:      d.Labels,
		Type:        d.Type,
		Connection: DBConnectionSummary{
			Host:     d.Connection.Host,
			Port:     d.Connection.Port,
			Username: d.Connection.Username,
			Database: d.Connection.Database,
			SSLMode:  d.Connection.SSLMode,
		},
		Readonly: d.Readonly,
		Limit:    d.Limit,
	}
}

// DatabasesFile is the root of databases.yaml.
type DatabasesFile struct {
	Databases []Database `yaml:"databases"`
}
