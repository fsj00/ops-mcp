package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsj00/ops-mcp/internal/model"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// LoadAll scans pluginsDir recursively for plugin.yml + main.js pairs.
func LoadAll(pluginsDir string, log *zap.Logger) ([]*model.PluginMeta, []error) {
	if log == nil {
		log = zap.NewNop()
	}
	var plugins []*model.PluginMeta
	var errs []error

	err := filepath.WalkDir(pluginsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "plugin.yml" {
			return nil
		}
		dir := filepath.Dir(path)
		p, loadErr := loadOne(dir)
		if loadErr != nil {
			log.Warn("skip plugin", zap.String("dir", dir), zap.Error(loadErr))
			errs = append(errs, fmt.Errorf("%s: %w", dir, loadErr))
			return nil
		}
		plugins = append(plugins, p)
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return plugins, errs
}

func loadOne(dir string) (*model.PluginMeta, error) {
	ymlPath := filepath.Join(dir, "plugin.yml")
	jsPath := filepath.Join(dir, "main.js")

	ymlData, err := os.ReadFile(ymlPath)
	if err != nil {
		return nil, err
	}
	jsData, err := os.ReadFile(jsPath)
	if err != nil {
		return nil, fmt.Errorf("missing main.js")
	}

	var meta model.PluginMeta
	if err := yaml.Unmarshal(ymlData, &meta); err != nil {
		return nil, fmt.Errorf("parse plugin.yml: %w", err)
	}
	if err := validateMeta(&meta); err != nil {
		return nil, err
	}
	meta.Path = dir
	meta.Script = string(jsData)
	return &meta, nil
}

func validateMeta(m *model.PluginMeta) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(m.Type) == "" {
		return fmt.Errorf("type is required")
	}
	if strings.TrimSpace(m.Target.Type) == "" {
		return fmt.Errorf("target.type is required")
	}
	if m.Runtime != "javascript" {
		return fmt.Errorf("runtime must be javascript")
	}
	if m.Input == nil {
		m.Input = map[string]model.PluginInputField{}
	}
	for name, field := range m.Input {
		if field.Type == "" {
			return fmt.Errorf("input.%s.type is required", name)
		}
	}
	return nil
}
