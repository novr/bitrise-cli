package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const LocalConfigFileName = ".br.yml"

type LocalConfig struct {
	App string `yaml:"app"`
}

// GitRoot returns the absolute path to the git repository root for dir.
func GitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("empty git root")
	}
	return root, nil
}

// LoadLocalConfig reads and parses a .br.yml file at path.
func LoadLocalConfig(path string) (*LocalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg LocalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// FindLocalConfig walks from cwd toward git root (parent direction only) and
// returns the first .br.yml with a non-empty app field.
func FindLocalConfig() (*LocalConfig, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	root, rootErr := GitRoot(cwd)
	inRepo := rootErr == nil

	dir := cwd
	for {
		path := filepath.Join(dir, LocalConfigFileName)
		if _, statErr := os.Stat(path); statErr == nil {
			cfg, loadErr := LoadLocalConfig(path)
			if loadErr != nil {
				return nil, "", loadErr
			}
			if cfg.App != "" {
				return cfg, path, nil
			}
		}

		if !inRepo {
			break
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, "", nil
}

// WriteLocalConfig writes app to .br.yml in dir.
func WriteLocalConfig(dir, app string) (string, error) {
	path := filepath.Join(dir, LocalConfigFileName)
	data, err := yaml.Marshal(&LocalConfig{App: app})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// DeprecatedDefaultApp reports whether config.yml still contains default_app.
func DeprecatedDefaultApp() (bool, error) {
	path, err := configPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	v, ok := raw["default_app"]
	if !ok || v == nil {
		return false, nil
	}
	s, _ := v.(string)
	return strings.TrimSpace(s) != "", nil
}
