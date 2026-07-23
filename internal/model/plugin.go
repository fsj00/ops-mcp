package model

import "time"

// PluginInputField describes one input parameter in plugin.yml.
type PluginInputField struct {
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Description string `yaml:"description" json:"description"`
}

// PluginTarget describes the connector target.
type PluginTarget struct {
	Type string `yaml:"type" json:"type"`
}

// PluginMeta is the parsed plugin.yml content plus load path.
type PluginMeta struct {
	Name        string                      `yaml:"name" json:"name"`
	Version     string                      `yaml:"version" json:"version"`
	Description string                      `yaml:"description" json:"description"`
	Type        string                      `yaml:"type" json:"type"`
	Target      PluginTarget                `yaml:"target" json:"target"`
	Input       map[string]PluginInputField `yaml:"input" json:"input"`
	Runtime     string                      `yaml:"runtime" json:"runtime"`
	Timeout     string                      `yaml:"timeout" json:"timeout"`
	Path        string                      `yaml:"-" json:"path"`
	Script      string                      `yaml:"-" json:"-"`
}

// TimeoutDuration returns the plugin timeout or the provided default.
func (p *PluginMeta) TimeoutDuration(defaultTimeout time.Duration) time.Duration {
	if p.Timeout == "" {
		return defaultTimeout
	}
	d, err := time.ParseDuration(p.Timeout)
	if err != nil || d <= 0 {
		return defaultTimeout
	}
	return d
}

// InputSchema converts plugin input into MCP JSON Schema.
func (p *PluginMeta) InputSchema() map[string]interface{} {
	properties := map[string]interface{}{}
	required := make([]string, 0)
	for name, field := range p.Input {
		prop := map[string]interface{}{
			"type": field.Type,
		}
		if field.Description != "" {
			prop["description"] = field.Description
		}
		properties[name] = prop
		if field.Required {
			required = append(required, name)
		}
	}
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
