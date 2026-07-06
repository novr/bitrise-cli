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

func gitRoot(dir string) (string, error) {
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

func LoadLocalConfig(path string) (*LocalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg LocalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.App = strings.TrimSpace(cfg.App) // YAML may carry whitespace-only as "set"
	return &cfg, nil
}

// FindLocalConfig returns the nearest .br.yml with a non-empty app, walking parents
// only up to the git root so monorepo root config is shared; outside a repo we stop
// at cwd to avoid picking up a parent directory's unrelated project.
func FindLocalConfig() (*LocalConfig, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	return findLocalConfigFrom(cwd)
}

func findLocalConfigFrom(startDir string) (*LocalConfig, string, error) {
	root, rootErr := gitRoot(startDir)
	inRepo := rootErr == nil

	dir := startDir
	for {
		path := filepath.Join(dir, LocalConfigFileName)
		if _, statErr := os.Stat(path); statErr == nil {
			cfg, loadErr := LoadLocalConfig(path)
			if loadErr != nil {
				return nil, "", loadErr
			}
			if cfg.App != "" { // empty app defers to a parent .br.yml
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

// DeprecatedDefaultAppValue reads config.yml as raw YAML because default_app was
// removed from Config; we still need to spot stale entries for doctor migration hints.
func DeprecatedDefaultAppValue() (slug string, ok bool, err error) {
	path, err := configPath()
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", false, err
	}
	v, exists := raw["default_app"]
	if !exists || v == nil {
		return "", false, nil
	}
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false, nil
	}
	return s, true, nil
}
