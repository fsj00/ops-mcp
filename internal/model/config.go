package model

// AppConfig is the ops-mcp.yaml root.
type AppConfig struct {
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Plugins  PluginsConfig  `mapstructure:"plugins" yaml:"plugins"`
	Config   PathsConfig    `mapstructure:"config" yaml:"config"`
	Defaults DefaultsConfig `mapstructure:"defaults" yaml:"defaults"`
	Log      LogConfig      `mapstructure:"log" yaml:"log"`
}

type ServerConfig struct {
	Host string     `mapstructure:"host" yaml:"host"`
	Port int        `mapstructure:"port" yaml:"port"`
	Auth AuthConfig `mapstructure:"auth" yaml:"auth"`
}

// AuthConfig protects HTTP / MCP endpoints with a shared token.
type AuthConfig struct {
	// Token is the shared secret. Empty disables auth (not recommended).
	// Env OPS_MCP_AUTH_TOKEN overrides this value when set.
	Token string `mapstructure:"token" yaml:"token"`
}

type PluginsConfig struct {
	Dir string `mapstructure:"dir" yaml:"dir"`
}

type PathsConfig struct {
	Hosts     string `mapstructure:"hosts" yaml:"hosts"`
	Databases string `mapstructure:"databases" yaml:"databases"`
	Redis     string `mapstructure:"redis" yaml:"redis"`
	Kafka     string `mapstructure:"kafka" yaml:"kafka"`
	Apis      string `mapstructure:"apis" yaml:"apis"`
	Commands  string `mapstructure:"commands" yaml:"commands"`
	SNMP      string `mapstructure:"snmp" yaml:"snmp"`
}

type DefaultsConfig struct {
	PluginTimeout string `mapstructure:"plugin_timeout" yaml:"plugin_timeout"`
}

type LogConfig struct {
	Level    string `mapstructure:"level" yaml:"level"`
	Encoding string `mapstructure:"encoding" yaml:"encoding"`
}
