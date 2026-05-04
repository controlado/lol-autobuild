package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild"
)

type Config struct {
	LogLevel       string               `yaml:"log_level"`
	Coachless      CoachlessConfig      `yaml:"coachless"`
	Auth           AuthConfig           `yaml:"auth"`
	EnvFile        EnvFileConfig        `yaml:"env_file"`
	Secrets        SecretsConfig        `yaml:"secrets"`
	Recommendation RecommendationConfig `yaml:"recommendation"`
	LCU            LCUConfig            `yaml:"lcu"`
	Sync           SyncConfig           `yaml:"sync"`
	Watch          WatchConfig          `yaml:"watch"`
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

type EnvFileConfig struct {
	Path string `yaml:"path"`
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

type SyncConfig struct {
	Patch              string `yaml:"patch"`
	PatchAdditionsMode string `yaml:"patch_additions_mode"`
	PatchAdditions     int    `yaml:"patch_additions"`
	LeagueTierPreset   string `yaml:"league_tier_preset"`
	ApplyItems         bool   `yaml:"apply_items"`
	ApplyRunes         bool   `yaml:"apply_runes"`
	ApplySpells        bool   `yaml:"apply_spells"`
	KeepFlash          bool   `yaml:"keep_flash"`
	DryRun             bool   `yaml:"dry_run"`
}

type WatchConfig struct {
	DebounceMillis       int `yaml:"debounce_millis"`
	ReconnectDelayMillis int `yaml:"reconnect_delay_millis"`
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
			MinOccurrence: 1000,
			TopItems:      10,
			TopSpells:     2,
		},
		LCU: LCUConfig{
			Enabled: false,
		},
		Sync: SyncConfig{
			PatchAdditionsMode: autobuild.PatchAdditionsModeAuto,
			PatchAdditions:     autobuild.PatchAdditionsDefault,
			LeagueTierPreset:   autobuild.LeagueTierPresetDefault,
			ApplyItems:         true,
			ApplyRunes:         true,
			ApplySpells:        true,
			KeepFlash:          true,
			DryRun:             false,
		},
		Watch: WatchConfig{
			DebounceMillis:       500,
			ReconnectDelayMillis: 1000,
		},
	}
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

	if c.Recommendation.TopItems < 0 {
		errs = append(errs, errors.New("recommendation.top_items must be >= 0"))
	}

	if c.Recommendation.TopSpells <= 0 {
		errs = append(errs, errors.New("recommendation.top_spells must be > 0"))
	}

	switch c.Sync.PatchAdditionsMode {
	case autobuild.PatchAdditionsModeAuto, autobuild.PatchAdditionsModeManual:
	default:
		errs = append(errs, fmt.Errorf("sync.patch_additions_mode must be %s or %s", autobuild.PatchAdditionsModeAuto, autobuild.PatchAdditionsModeManual))
	}

	if c.Sync.PatchAdditions < 0 || c.Sync.PatchAdditions > autobuild.PatchAdditionsMax {
		errs = append(errs, fmt.Errorf("sync.patch_additions must be between 0 and %d", autobuild.PatchAdditionsMax))
	}

	switch c.Sync.LeagueTierPreset {
	case autobuild.LeagueTierPresetGoldPlus,
		autobuild.LeagueTierPresetPlatinumPlus,
		autobuild.LeagueTierPresetEmeraldPlus,
		autobuild.LeagueTierPresetDiamondPlus,
		autobuild.LeagueTierPresetMasterPlus:
	default:
		errs = append(errs, errors.New("sync.league_tier_preset is invalid"))
	}

	if c.Watch.DebounceMillis <= 0 {
		errs = append(errs, errors.New("watch.debounce_millis must be > 0"))
	}

	if c.Watch.ReconnectDelayMillis <= 0 {
		errs = append(errs, errors.New("watch.reconnect_delay_millis must be > 0"))
	}

	return errors.Join(errs...)
}
