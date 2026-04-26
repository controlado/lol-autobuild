package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ConfigStore struct {
	path string
}

func NewConfigStore(path string) (*ConfigStore, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be blank")
	}
	return &ConfigStore{path: path}, nil
}

func (cs *ConfigStore) Load() (Config, error) {
	cfg := Defaults()

	raw, err := os.ReadFile(cs.path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cs *ConfigStore) Save(new Config) error {
	if err := new.Validate(); err != nil {
		return err
	}

	raw, err := yaml.Marshal(new)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	mode := os.FileMode(0o600)
	if info, err := os.Stat(cs.path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config %s: %w", cs.path, err)
	}

	if err := os.WriteFile(cs.path, raw, mode); err != nil {
		return fmt.Errorf("write config %s: %w", cs.path, err)
	}

	return nil
}
