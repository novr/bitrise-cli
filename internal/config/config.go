package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token string `yaml:"token"`
}

func configPath() (string, error) {
	if dir := os.Getenv("BR_CONFIG_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		return filepath.Join(abs, "config.yml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "br", "config.yml"), nil
}

// Path returns the location of the config file (for display).
func Path() (string, error) {
	return configPath()
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// tokenEnvVars are the environment variables checked for a token, in order.
// BITRISE_API_TOKEN is the more explicit name; BITRISE_TOKEN is kept for compat.
var tokenEnvVars = []string{"BITRISE_API_TOKEN", "BITRISE_TOKEN"}

func GetToken() (string, error) {
	for _, name := range tokenEnvVars {
		if token := os.Getenv(name); token != "" {
			return token, nil
		}
	}
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Token == "" {
		return "", fmt.Errorf("not authenticated: run 'br auth login' or set BITRISE_API_TOKEN")
	}
	return cfg.Token, nil
}
