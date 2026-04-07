package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogLevel       string               `yaml:"log_level"`
	Coachless      CoachlessConfig      `yaml:"coachless"`
	Auth           AuthConfig           `yaml:"auth"`
	Secrets        SecretsConfig        `yaml:"secrets"`
	Recommendation RecommendationConfig `yaml:"recommendation"`
	LCU            LCUConfig            `yaml:"lcu"`
}

type CoachlessConfig struct {
	APIBaseURL     string `yaml:"api_base_url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type AuthConfig struct {
	AutoEnabled           bool `yaml:"auto_enabled"`
	ManualFallbackEnabled bool `yaml:"manual_fallback_enabled"`
	TokenSkewSeconds      int  `yaml:"token_skew_seconds"`
}

type SecretsConfig struct {
	ServiceName string `yaml:"service_name"`
}

type RecommendationConfig struct {
	MinOccurrence int `yaml:"min_occurrence"`
	TopItems      int `yaml:"top_items"`
	TopSpells     int `yaml:"top_spells"`
}

type LCUConfig struct {
	Enabled      bool   `yaml:"enabled"`
	LockfilePath string `yaml:"lockfile_path"`
}

func Defaults() Config {
	return Config{
		LogLevel: "info",
		Coachless: CoachlessConfig{
			APIBaseURL:     "https://api.coachless.gg",
			TimeoutSeconds: 20,
		},
		Auth: AuthConfig{
			AutoEnabled:           true,
			ManualFallbackEnabled: true,
			TokenSkewSeconds:      30,
		},
		Secrets: SecretsConfig{
			ServiceName: "lol-autobuild",
		},
		Recommendation: RecommendationConfig{
			MinOccurrence: 100,
			TopItems:      6,
			TopSpells:     2,
		},
		LCU: LCUConfig{
			Enabled: false,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	raw, err := os.ReadFile(path)
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

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Coachless.APIBaseURL) == "" {
		errs = append(errs, errors.New("coachless.api_base_url is required"))
	}

	if c.Coachless.TimeoutSeconds <= 0 {
		errs = append(errs, errors.New("coachless.timeout_seconds must be > 0"))
	}

	if c.Auth.TokenSkewSeconds < 0 {
		errs = append(errs, errors.New("auth.token_skew_seconds must be >= 0"))
	}

	if !c.Auth.AutoEnabled && !c.Auth.ManualFallbackEnabled {
		errs = append(errs, errors.New("at least one auth mode must be enabled"))
	}

	if strings.TrimSpace(c.Secrets.ServiceName) == "" {
		errs = append(errs, errors.New("secrets.service_name is required"))
	}

	if c.Recommendation.MinOccurrence < 0 {
		errs = append(errs, errors.New("recommendation.min_occurrence must be >= 0"))
	}

	if c.Recommendation.TopItems <= 0 {
		errs = append(errs, errors.New("recommendation.top_items must be > 0"))
	}

	if c.Recommendation.TopSpells <= 0 {
		errs = append(errs, errors.New("recommendation.top_spells must be > 0"))
	}

	return errors.Join(errs...)
}
