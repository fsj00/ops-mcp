package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// CommandsFile is the root of commands.yaml.
type CommandsFile struct {
	Commands []Command `yaml:"commands"`
}

// PathList is one or more absolute executable paths (first available wins at load).
// YAML may be a string or a sequence of strings.
type PathList []string

// UnmarshalYAML accepts `path: /bin/ping` or `path: [/sbin/ping, /bin/ping]`.
func (p *PathList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		*p = PathList{s}
		return nil
	case yaml.SequenceNode:
		var ss []string
		if err := value.Decode(&ss); err != nil {
			return err
		}
		*p = PathList(ss)
		return nil
	case 0:
		*p = nil
		return nil
	default:
		return fmt.Errorf("path must be a string or array of strings")
	}
}

// Command is a whitelisted local executable.
type Command struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Paths       PathList `yaml:"path" json:"-"`
	// Path is the resolved absolute executable chosen at load/reload time.
	Path string `yaml:"-" json:"path"`
}

// CommandSummary is the agent-facing command metadata.
type CommandSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// ToSummary returns a CommandSummary.
func (c Command) ToSummary() CommandSummary {
	return CommandSummary{
		Name:        c.Name,
		Description: c.Description,
		Path:        c.Path,
	}
}
