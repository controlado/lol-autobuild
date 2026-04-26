package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	t.Parallel()

	var (
		dir  = t.TempDir()
		path = filepath.Join(dir, "config.yaml")
		raw  = `
log_level: debug
coachless:
  api_base_url: https://api.coachless.gg
  timeout_seconds: 15
auth:
  auto_enabled: true
  manual_fallback_enabled: true
  token_skew_seconds: 20
secrets:
  service_name: lol-autobuild
recommendation:
  min_occurrence: 50
  top_items: 5
  top_spells: 2
lcu:
  enabled: false
sync:
  patch: "16.7"
  apply_items: true
  apply_runes: false
  apply_spells: true
  dry_run: false
env_file:
  path: .env.local
`
	)

	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configStore, err := NewConfigStore(path)
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	cfg, err := configStore.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected log level: %s", cfg.LogLevel)
	}

	if cfg.Recommendation.TopItems != 5 {
		t.Fatalf("unexpected top items: %d", cfg.Recommendation.TopItems)
	}

	if cfg.EnvFile.Path != ".env.local" {
		t.Fatalf("unexpected env file path: %s", cfg.EnvFile.Path)
	}

	if cfg.Sync.Patch != "16.7" {
		t.Fatalf("unexpected sync patch: %s", cfg.Sync.Patch)
	}

	if !cfg.Sync.ApplyItems || cfg.Sync.ApplyRunes || !cfg.Sync.ApplySpells || cfg.Sync.DryRun {
		t.Fatalf("unexpected sync config: %#v", cfg.Sync)
	}
}

func TestLoadAppliesSyncDefaults(t *testing.T) {
	t.Parallel()

	var (
		dir  = t.TempDir()
		path = filepath.Join(dir, "config.yaml")
		raw  = `
log_level: debug
coachless:
  api_base_url: https://api.coachless.gg
  timeout_seconds: 15
auth:
  auto_enabled: true
  manual_fallback_enabled: true
  token_skew_seconds: 20
secrets:
  service_name: lol-autobuild
recommendation:
  min_occurrence: 50
  top_items: 5
  top_spells: 2
lcu:
  enabled: false
`
	)

	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configStore, err := NewConfigStore(path)
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	cfg, err := configStore.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Sync.Patch != "" {
		t.Fatalf("expected empty default patch, got %q", cfg.Sync.Patch)
	}
	if !cfg.Sync.ApplyItems || !cfg.Sync.ApplyRunes || !cfg.Sync.ApplySpells || !cfg.Sync.DryRun {
		t.Fatalf("unexpected sync defaults: %#v", cfg.Sync)
	}
}

func TestSaveWritesConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Defaults()
	cfg.LCU.Enabled = true
	cfg.Sync.Patch = "16.8"
	cfg.Sync.ApplyRunes = false
	cfg.Sync.DryRun = false

	configStore, err := NewConfigStore(path)
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	if err := configStore.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := configStore.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reloaded.LCU.Enabled {
		t.Fatalf("expected lcu.enabled to persist")
	}
	if reloaded.Sync.Patch != "16.8" || reloaded.Sync.ApplyRunes || reloaded.Sync.DryRun {
		t.Fatalf("unexpected saved sync config: %#v", reloaded.Sync)
	}
}

func TestLoadAcceptsMissingOrEmptyEnvFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing env_file section",
			raw: `
log_level: debug
coachless:
  api_base_url: https://api.coachless.gg
  timeout_seconds: 15
auth:
  auto_enabled: true
  manual_fallback_enabled: true
  token_skew_seconds: 20
secrets:
  service_name: lol-autobuild
recommendation:
  min_occurrence: 50
  top_items: 5
  top_spells: 2
lcu:
  enabled: false
`,
		},
		{
			name: "empty env_file path",
			raw: `
log_level: debug
coachless:
  api_base_url: https://api.coachless.gg
  timeout_seconds: 15
auth:
  auto_enabled: true
  manual_fallback_enabled: true
  token_skew_seconds: 20
env_file:
  path: ""
secrets:
  service_name: lol-autobuild
recommendation:
  min_occurrence: 50
  top_items: 5
  top_spells: 2
lcu:
  enabled: false
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")

			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			configStore, err := NewConfigStore(path)
			if err != nil {
				t.Fatalf("NewConfigStore() error = %v", err)
			}

			if _, err := configStore.Load(); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestValidateFailsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.Coachless.APIBaseURL = ""
	cfg.Recommendation.TopSpells = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "coachless.api_base_url") {
		t.Fatalf("expected base url error, got: %s", msg)
	}

	if !strings.Contains(msg, "recommendation.top_spells") {
		t.Fatalf("expected top_spells error, got: %s", msg)
	}
}

func TestValidateFailsOnInvalidWatchConfig(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.Watch.DebounceMillis = 0
	cfg.Watch.ReconnectDelayMillis = -1

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "watch.debounce_millis") {
		t.Fatalf("expected debounce validation error, got: %s", msg)
	}

	if !strings.Contains(msg, "watch.reconnect_delay_millis") {
		t.Fatalf("expected reconnect_delay validation error, got: %s", msg)
	}
}
