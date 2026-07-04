package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token      string `yaml:"token"`
	DefaultApp string `yaml:"default_app,omitempty"`
}

func configPath() (string, error) {
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

func GetToken() (string, error) {
	if token := os.Getenv("BITRISE_TOKEN"); token != "" {
		return token, nil
	}
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Token == "" {
		return "", fmt.Errorf("not authenticated: run 'br auth login' or set BITRISE_TOKEN")
	}
	return cfg.Token, nil
}
