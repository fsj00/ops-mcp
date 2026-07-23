package plugin

import (
	"fmt"
	"sync"

	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
)

// Manager holds the currently loaded plugins.
type Manager struct {
	mu         sync.RWMutex
	pluginsDir string
	plugins    map[string]*model.PluginMeta
	log        *zap.Logger
	reserved   func() []string // e.g. OpenAPI tool names
}

func NewManager(pluginsDir string, log *zap.Logger) *Manager {
	if log == nil {
		log = zap.NewNop()
	}
	return &Manager{
		pluginsDir: pluginsDir,
		plugins:    map[string]*model.PluginMeta{},
		log:        log,
	}
}

// SetReservedNamesProvider supplies names that must not collide with disk plugins (e.g. OpenAPI tools).
func (m *Manager) SetReservedNamesProvider(fn func() []string) {
	m.reserved = fn
}

// Load scans and replaces the plugin set. On failure keeps previous set.
func (m *Manager) Load() (int, error) {
	loaded, loadErrs := LoadAll(m.pluginsDir, m.log)
	next := make(map[string]*model.PluginMeta, len(loaded))
	reserved := map[string]struct{}{}
	if m.reserved != nil {
		for _, n := range m.reserved() {
			reserved[n] = struct{}{}
		}
	}
	for _, p := range loaded {
		if _, exists := next[p.Name]; exists {
			return 0, fmt.Errorf("duplicate plugin name %q", p.Name)
		}
		if _, clash := reserved[p.Name]; clash {
			return 0, fmt.Errorf("plugin %q conflicts with openapi tool", p.Name)
		}
		next[p.Name] = p
	}
	if len(next) == 0 && len(loadErrs) > 0 {
		return 0, fmt.Errorf("no plugins loaded: %v", loadErrs)
	}

	m.mu.Lock()
	m.plugins = next
	m.mu.Unlock()

	m.log.Info("plugins loaded", zap.Int("count", len(next)), zap.Int("errors", len(loadErrs)))
	return len(next), nil
}

// Names returns all loaded plugin tool names.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.plugins))
	for n := range m.plugins {
		out = append(out, n)
	}
	return out
}

// Get returns a plugin by tool name.
func (m *Manager) Get(name string) (*model.PluginMeta, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

// List returns all loaded plugins.
func (m *Manager) List() []*model.PluginMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*model.PluginMeta, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, p)
	}
	return out
}

// Count returns the number of loaded plugins.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}
