package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Manager loads and serves application and resource configuration.
type Manager struct {
	mu              sync.RWMutex
	app             model.AppConfig
	hosts           map[string]model.Host
	databases       map[string]model.Database
	redis           map[string]model.RedisInstance
	kafka           map[string]model.KafkaInstance
	apis            map[string]model.APIService
	commands        map[string]model.Command
	snmpCreds       map[string]model.SNMPCredential
	snmpDevices     map[string]model.SNMPDevice
	snmpDefaults    model.SNMPDefaults
	configPath      string
	log             *zap.Logger
	commandWarnings []string
}

// Load reads ops-mcp.yaml and linked resource files.
func Load(configPath string) (*Manager, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var app model.AppConfig
	if err := v.Unmarshal(&app); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	applyDefaults(&app)

	m := &Manager{
		app:        app,
		hosts:      map[string]model.Host{},
		databases:  map[string]model.Database{},
		redis:      map[string]model.RedisInstance{},
		kafka:      map[string]model.KafkaInstance{},
		apis:        map[string]model.APIService{},
		commands:    map[string]model.Command{},
		snmpCreds:   map[string]model.SNMPCredential{},
		snmpDevices: map[string]model.SNMPDevice{},
		configPath:  configPath,
	}
	if err := m.reloadResources(); err != nil {
		return nil, err
	}
	return m, nil
}

func applyDefaults(app *model.AppConfig) {
	if app.Server.Host == "" {
		app.Server.Host = "0.0.0.0"
	}
	if app.Server.Port == 0 {
		app.Server.Port = 20267
	}
	if app.Plugins.Dir == "" {
		app.Plugins.Dir = "./plugins"
	}
	if app.Config.Hosts == "" {
		app.Config.Hosts = "./config/hosts.yaml"
	}
	if app.Config.Databases == "" {
		app.Config.Databases = "./config/databases.yaml"
	}
	if app.Config.Redis == "" {
		app.Config.Redis = "./config/redis.yaml"
	}
	if app.Config.Kafka == "" {
		app.Config.Kafka = "./config/kafka.yaml"
	}
	if app.Config.Apis == "" {
		app.Config.Apis = "./config/apis.yaml"
	}
	if app.Config.Commands == "" {
		app.Config.Commands = "./config/commands.yaml"
	}
	if app.Config.SNMP == "" {
		app.Config.SNMP = "./config/snmp.yaml"
	}
	if app.Defaults.PluginTimeout == "" {
		app.Defaults.PluginTimeout = "30s"
	}
	if app.Log.Level == "" {
		app.Log.Level = "info"
	}
	if app.Log.Encoding == "" {
		app.Log.Encoding = "json"
	}
	if tok := os.Getenv("OPS_MCP_AUTH_TOKEN"); tok != "" {
		app.Server.Auth.Token = tok
	}
}

func (m *Manager) reloadResources() error {
	hosts, err := loadHosts(m.app.Config.Hosts)
	if err != nil {
		return err
	}
	dbs, err := loadDatabases(m.app.Config.Databases)
	if err != nil {
		return err
	}
	rds, err := loadRedis(m.app.Config.Redis)
	if err != nil {
		return err
	}
	kfk, err := loadKafka(m.app.Config.Kafka)
	if err != nil {
		return err
	}
	apis, err := loadAPIs(m.app.Config.Apis)
	if err != nil {
		return err
	}
	commands, warnings, err := loadCommands(m.app.Config.Commands)
	if err != nil {
		return err
	}
	creds, devices, defaults, err := loadSNMP(m.app.Config.SNMP)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts = hosts
	m.databases = dbs
	m.redis = rds
	m.kafka = kfk
	m.apis = apis
	m.commands = commands
	m.snmpCreds = creds
	m.snmpDevices = devices
	m.snmpDefaults = defaults
	m.commandWarnings = warnings
	if m.log != nil {
		for _, w := range warnings {
			m.log.Warn(w)
		}
		m.commandWarnings = nil
	}
	return nil
}

// SetLogger attaches a logger and flushes any pending command-load warnings
// (Load runs before the process logger is created).
func (m *Manager) SetLogger(log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = log
	for _, w := range m.commandWarnings {
		log.Warn(w)
	}
	m.commandWarnings = nil
}

func loadHosts(path string) (map[string]model.Host, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hosts %s: %w", path, err)
	}
	var file model.HostsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse hosts %s: %w", path, err)
	}
	out := make(map[string]model.Host, len(file.Hosts))
	for _, h := range file.Hosts {
		if h.Name == "" {
			return nil, fmt.Errorf("hosts.yaml: host missing name")
		}
		if h.Address.Port == 0 {
			h.Address.Port = 22
		}
		if _, exists := out[h.Name]; exists {
			return nil, fmt.Errorf("hosts.yaml: duplicate host name %q", h.Name)
		}
		out[h.Name] = h
	}
	return out, nil
}

func loadDatabases(path string) (map[string]model.Database, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.Database{}, nil
		}
		return nil, fmt.Errorf("read databases %s: %w", path, err)
	}
	var file model.DatabasesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse databases %s: %w", path, err)
	}
	out := make(map[string]model.Database, len(file.Databases))
	for _, db := range file.Databases {
		if db.Name == "" {
			return nil, fmt.Errorf("databases.yaml: database missing name")
		}
		if _, exists := out[db.Name]; exists {
			return nil, fmt.Errorf("databases.yaml: duplicate database name %q", db.Name)
		}
		if db.Limit <= 0 {
			db.Limit = DefaultQueryLimit
		}
		if db.Connection.Port == 0 {
			switch db.Type {
			case "mysql":
				db.Connection.Port = 3306
			case "postgresql":
				db.Connection.Port = 5432
			}
		}
		out[db.Name] = db
	}
	return out, nil
}

// DefaultQueryLimit is used when databases.yaml / redis.yaml / kafka.yaml omits limit.
const DefaultQueryLimit = 1000

func loadKafka(path string) (map[string]model.KafkaInstance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.KafkaInstance{}, nil
		}
		return nil, fmt.Errorf("read kafka %s: %w", path, err)
	}
	var file model.KafkaFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse kafka %s: %w", path, err)
	}
	out := make(map[string]model.KafkaInstance, len(file.Kafka))
	for _, k := range file.Kafka {
		if k.Name == "" {
			return nil, fmt.Errorf("kafka.yaml: kafka missing name")
		}
		if _, exists := out[k.Name]; exists {
			return nil, fmt.Errorf("kafka.yaml: duplicate kafka name %q", k.Name)
		}
		if k.Limit <= 0 {
			k.Limit = DefaultQueryLimit
		}
		if len(k.Connection.Brokers) == 0 {
			return nil, fmt.Errorf("kafka.yaml: kafka %q missing connection.brokers", k.Name)
		}
		out[k.Name] = k
	}
	return out, nil
}

func loadRedis(path string) (map[string]model.RedisInstance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.RedisInstance{}, nil
		}
		return nil, fmt.Errorf("read redis %s: %w", path, err)
	}
	var file model.RedisFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse redis %s: %w", path, err)
	}
	out := make(map[string]model.RedisInstance, len(file.Redis))
	for _, r := range file.Redis {
		if r.Name == "" {
			return nil, fmt.Errorf("redis.yaml: redis missing name")
		}
		if _, exists := out[r.Name]; exists {
			return nil, fmt.Errorf("redis.yaml: duplicate redis name %q", r.Name)
		}
		if r.Limit <= 0 {
			r.Limit = DefaultQueryLimit
		}
		if r.Connection.Port == 0 {
			r.Connection.Port = 6379
		}
		out[r.Name] = r
	}
	return out, nil
}

// App returns a copy of the application config.
func (m *Manager) App() model.AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.app
}

// DefaultPluginTimeout returns the global plugin timeout.
func (m *Manager) DefaultPluginTimeout() time.Duration {
	d, err := time.ParseDuration(m.App().Defaults.PluginTimeout)
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

// GetHost looks up a host by name.
func (m *Manager) GetHost(name string) (model.Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hosts[name]
	if !ok {
		return model.Host{}, fmt.Errorf("host %q not found", name)
	}
	return h, nil
}

// GetDatabase looks up a database by name.
func (m *Manager) GetDatabase(name string) (model.Database, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	db, ok := m.databases[name]
	if !ok {
		return model.Database{}, fmt.Errorf("database %q not found", name)
	}
	return db, nil
}

// GetRedis looks up a Redis instance by name.
func (m *Manager) GetRedis(name string) (model.RedisInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.redis[name]
	if !ok {
		return model.RedisInstance{}, fmt.Errorf("redis %q not found", name)
	}
	return r, nil
}

// GetKafka looks up a Kafka instance by name.
func (m *Manager) GetKafka(name string) (model.KafkaInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.kafka[name]
	if !ok {
		return model.KafkaInstance{}, fmt.Errorf("kafka %q not found", name)
	}
	return k, nil
}

// GetAPI looks up an API service by name.
func (m *Manager) GetAPI(name string) (model.APIService, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.apis[name]
	if !ok {
		return model.APIService{}, fmt.Errorf("api %q not found", name)
	}
	return a, nil
}

// GetCommand looks up a whitelisted local command by name.
func (m *Manager) GetCommand(name string) (model.Command, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.commands[name]
	if !ok {
		return model.Command{}, fmt.Errorf("command %q not found", name)
	}
	return c, nil
}

// GetSNMPDevice looks up an SNMP device by name.
func (m *Manager) GetSNMPDevice(name string) (model.SNMPDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.snmpDevices[name]
	if !ok {
		return model.SNMPDevice{}, fmt.Errorf("snmp device %q not found", name)
	}
	return d, nil
}

// GetSNMPCredential looks up a named SNMP credential profile.
func (m *Manager) GetSNMPCredential(name string) (model.SNMPCredential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.snmpCreds[name]
	if !ok {
		return model.SNMPCredential{}, fmt.Errorf("snmp credential %q not found", name)
	}
	return c, nil
}

// ResolveSNMPAuth returns the effective SNMPAuth for a device (credential ref or inline).
func (m *Manager) ResolveSNMPAuth(deviceName string) (model.SNMPAuth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.snmpDevices[deviceName]
	if !ok {
		return model.SNMPAuth{}, fmt.Errorf("snmp device %q not found", deviceName)
	}
	if d.Credential != "" {
		c, ok := m.snmpCreds[d.Credential]
		if !ok {
			return model.SNMPAuth{}, fmt.Errorf("snmp credential %q not found", d.Credential)
		}
		return c.SNMPAuth, nil
	}
	if d.Auth != nil {
		return *d.Auth, nil
	}
	return model.SNMPAuth{}, fmt.Errorf("snmp device %q has no credential or auth", deviceName)
}

// SNMPDefaults returns file-level SNMP defaults (copy).
func (m *Manager) SNMPDefaults() model.SNMPDefaults {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snmpDefaults
}

// SNMPDeviceListOptions filters and pages SNMP device summaries.
type SNMPDeviceListOptions struct {
	Labels map[string]string
	Limit  int
	Offset int
}

// DefaultSNMPListLimit is used when list opts omit limit or set it <= 0.
const DefaultSNMPListLimit = 100

// ListSNMPDeviceSummaries returns safe SNMP device metadata (no secrets).
// Labels filter is exact match on all provided keys. Default limit is 100.
func (m *Manager) ListSNMPDeviceSummaries(opts SNMPDeviceListOptions) []model.SNMPDeviceSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.SNMPDeviceSummary, 0, len(m.snmpDevices))
	for _, d := range m.snmpDevices {
		if !labelsMatch(d.Labels, opts.Labels) {
			continue
		}
		out = append(out, d.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultSNMPListLimit
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(out) {
		return []model.SNMPDeviceSummary{}
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end]
}

func labelsMatch(have, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

// ListAPIs returns a copy of all loaded API services (includes headers for connector use).
func (m *Manager) ListAPIs() []model.APIService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.APIService, 0, len(m.apis))
	for _, a := range m.apis {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListHostSummaries returns safe host metadata for Agents (no secrets).
func (m *Manager) ListHostSummaries() []model.HostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.HostSummary, 0, len(m.hosts))
	for _, h := range m.hosts {
		out = append(out, h.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListDatabaseSummaries returns safe database metadata (no passwords).
func (m *Manager) ListDatabaseSummaries() []model.DatabaseSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.DatabaseSummary, 0, len(m.databases))
	for _, db := range m.databases {
		out = append(out, db.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListRedisSummaries returns safe Redis metadata (no passwords).
func (m *Manager) ListRedisSummaries() []model.RedisSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.RedisSummary, 0, len(m.redis))
	for _, r := range m.redis {
		out = append(out, r.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListKafkaSummaries returns safe Kafka metadata (no passwords / PEM).
func (m *Manager) ListKafkaSummaries() []model.KafkaSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.KafkaSummary, 0, len(m.kafka))
	for _, k := range m.kafka {
		out = append(out, k.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListAPISummaries returns safe API metadata (no header values). ToolCount is 0 here;
// the OpenAPI registry fills tool_count when exposing to agents.
func (m *Manager) ListAPISummaries() []model.APISummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.APISummary, 0, len(m.apis))
	for _, a := range m.apis {
		out = append(out, a.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListCommandSummaries returns whitelisted local command metadata.
func (m *Manager) ListCommandSummaries() []model.CommandSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.CommandSummary, 0, len(m.commands))
	for _, c := range m.commands {
		out = append(out, c.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AuthToken returns the configured auth token (may be empty = auth disabled).
func (m *Manager) AuthToken() string {
	return m.App().Server.Auth.Token
}

// ReloadConfig reloads hosts.yaml, databases.yaml, redis.yaml, kafka.yaml,
// apis.yaml, commands.yaml and snmp.yaml.
func (m *Manager) ReloadConfig() error {
	return m.reloadResources()
}

// DefaultSNMPTimeout / retries / bulk / walk caps when snmp.yaml defaults omit them.
const (
	DefaultSNMPTimeout        = "5s"
	DefaultSNMPRetries        = 1
	DefaultSNMPMaxRepetitions = 25
	DefaultSNMPWalkMaxOIDs    = 1000
)

func loadSNMP(path string) (map[string]model.SNMPCredential, map[string]model.SNMPDevice, model.SNMPDefaults, error) {
	emptyCreds := map[string]model.SNMPCredential{}
	emptyDevs := map[string]model.SNMPDevice{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyCreds, emptyDevs, defaultSNMPDefaults(), nil
		}
		return nil, nil, model.SNMPDefaults{}, fmt.Errorf("read snmp %s: %w", path, err)
	}
	var file model.SNMPFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, model.SNMPDefaults{}, fmt.Errorf("parse snmp %s: %w", path, err)
	}
	defaults := applySNMPDefaults(file.Defaults)

	creds := make(map[string]model.SNMPCredential, len(file.Credentials))
	for _, c := range file.Credentials {
		if c.Name == "" {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf("snmp.yaml: credential missing name")
		}
		if _, exists := creds[c.Name]; exists {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf("snmp.yaml: duplicate credential name %q", c.Name)
		}
		if err := validateSNMPAuth(c.SNMPAuth, fmt.Sprintf("credential %q", c.Name)); err != nil {
			return nil, nil, model.SNMPDefaults{}, err
		}
		creds[c.Name] = c
	}

	devs := make(map[string]model.SNMPDevice, len(file.Devices))
	for _, d := range file.Devices {
		if d.Name == "" {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf("snmp.yaml: device missing name")
		}
		if _, exists := devs[d.Name]; exists {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf("snmp.yaml: duplicate device name %q", d.Name)
		}
		if d.Address.Host == "" {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf("snmp.yaml: device %q missing address.host", d.Name)
		}
		if d.Address.Port == 0 {
			d.Address.Port = 161
		}
		hasCred := d.Credential != ""
		hasAuth := d.Auth != nil
		if hasCred == hasAuth {
			return nil, nil, model.SNMPDefaults{}, fmt.Errorf(
				"snmp.yaml: device %q must set exactly one of credential or auth", d.Name)
		}
		if hasCred {
			if _, ok := creds[d.Credential]; !ok {
				return nil, nil, model.SNMPDefaults{}, fmt.Errorf(
					"snmp.yaml: device %q references unknown credential %q", d.Name, d.Credential)
			}
		} else if err := validateSNMPAuth(*d.Auth, fmt.Sprintf("device %q auth", d.Name)); err != nil {
			return nil, nil, model.SNMPDefaults{}, err
		}
		devs[d.Name] = d
	}
	return creds, devs, defaults, nil
}

func defaultSNMPDefaults() model.SNMPDefaults {
	return model.SNMPDefaults{
		Timeout:        DefaultSNMPTimeout,
		Retries:        DefaultSNMPRetries,
		MaxRepetitions: DefaultSNMPMaxRepetitions,
		WalkMaxOIDs:    DefaultSNMPWalkMaxOIDs,
	}
}

func applySNMPDefaults(d model.SNMPDefaults) model.SNMPDefaults {
	out := defaultSNMPDefaults()
	if d.Timeout != "" {
		out.Timeout = d.Timeout
	}
	if d.Retries > 0 {
		out.Retries = d.Retries
	}
	if d.MaxRepetitions > 0 {
		out.MaxRepetitions = d.MaxRepetitions
	}
	if d.WalkMaxOIDs > 0 {
		out.WalkMaxOIDs = d.WalkMaxOIDs
	}
	return out
}

func validateSNMPAuth(a model.SNMPAuth, where string) error {
	switch strings.ToLower(strings.TrimSpace(a.Version)) {
	case "2c":
		if a.Community == "" {
			return fmt.Errorf("snmp.yaml: %s version 2c requires community", where)
		}
	case "3":
		if a.Username == "" {
			return fmt.Errorf("snmp.yaml: %s version 3 requires username", where)
		}
		level := strings.ToLower(strings.TrimSpace(a.SecurityLevel))
		if level == "" {
			level = "noauthnopriv"
		}
		switch level {
		case "noauthnopriv":
		case "authnopriv":
			if a.AuthPassword == "" {
				return fmt.Errorf("snmp.yaml: %s authNoPriv requires auth_password", where)
			}
		case "authpriv":
			if a.AuthPassword == "" {
				return fmt.Errorf("snmp.yaml: %s authPriv requires auth_password", where)
			}
			if a.PrivPassword == "" {
				return fmt.Errorf("snmp.yaml: %s authPriv requires priv_password", where)
			}
		default:
			return fmt.Errorf("snmp.yaml: %s invalid security_level %q", where, a.SecurityLevel)
		}
	default:
		return fmt.Errorf("snmp.yaml: %s version must be 2c or 3, got %q", where, a.Version)
	}
	return nil
}

func loadCommands(path string) (map[string]model.Command, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.Command{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read commands %s: %w", path, err)
	}
	var file model.CommandsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("parse commands %s: %w", path, err)
	}
	out := make(map[string]model.Command, len(file.Commands))
	var warnings []string
	for _, c := range file.Commands {
		if c.Name == "" {
			return nil, nil, fmt.Errorf("commands.yaml: command missing name")
		}
		candidates := normalizePathCandidates(c.Paths)
		if len(candidates) == 0 {
			return nil, nil, fmt.Errorf("commands.yaml: command %q missing path", c.Name)
		}
		for _, p := range candidates {
			if !filepath.IsAbs(p) {
				return nil, nil, fmt.Errorf("commands.yaml: command %q path must be absolute, got %q", c.Name, p)
			}
		}
		if _, exists := out[c.Name]; exists {
			return nil, nil, fmt.Errorf("commands.yaml: duplicate command name %q", c.Name)
		}
		resolved := resolveCommandPath(candidates)
		if resolved == "" {
			warnings = append(warnings, fmt.Sprintf(
				"commands.yaml: command %q has no available path among %v", c.Name, candidates,
			))
			continue
		}
		c.Paths = candidates
		c.Path = resolved
		out[c.Name] = c
	}
	return out, warnings, nil
}

func normalizePathCandidates(paths model.PathList) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}

// resolveCommandPath returns the first existing executable path.
func resolveCommandPath(candidates []string) string {
	for _, p := range candidates {
		if commandPathAvailable(p) {
			return p
		}
	}
	return ""
}

func commandPathAvailable(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func loadAPIs(path string) (map[string]model.APIService, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.APIService{}, nil
		}
		return nil, fmt.Errorf("read apis %s: %w", path, err)
	}
	var file model.APIsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse apis %s: %w", path, err)
	}
	out := make(map[string]model.APIService, len(file.APIs))
	for i, a := range file.APIs {
		if a.Name == "" {
			return nil, fmt.Errorf("apis.yaml: api missing name")
		}
		if _, exists := out[a.Name]; exists {
			return nil, fmt.Errorf("apis.yaml: duplicate api name %q", a.Name)
		}
		prefix := fmt.Sprintf("apis[%s]", a.Name)
		expanded, err := expandAPIService(a, prefix)
		if err != nil {
			return nil, err
		}
		if expanded.OpenAPI.Path == "" {
			return nil, fmt.Errorf("%s.openapi.path: required", prefix)
		}
		if expanded.Endpoint.BaseURL == "" {
			return nil, fmt.Errorf("%s.endpoint.base_url: required", prefix)
		}
		file.APIs[i] = expanded
		out[expanded.Name] = expanded
	}
	return out, nil
}

func expandAPIService(a model.APIService, prefix string) (model.APIService, error) {
	var err error
	a.OpenAPI.Path, err = expandEnvStrict(a.OpenAPI.Path, prefix+".openapi.path")
	if err != nil {
		return a, err
	}
	a.Endpoint.BaseURL, err = expandEnvStrict(a.Endpoint.BaseURL, prefix+".endpoint.base_url")
	if err != nil {
		return a, err
	}
	if len(a.Headers) > 0 {
		headers := make(map[string]string, len(a.Headers))
		for k, v := range a.Headers {
			expanded, err := expandEnvStrict(v, fmt.Sprintf("%s.headers.%s", prefix, k))
			if err != nil {
				return a, err
			}
			headers[k] = expanded
		}
		a.Headers = headers
	}
	return a, nil
}
