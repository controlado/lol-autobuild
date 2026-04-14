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
env_file:
  path: .env.local
`
	)

	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
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

			if _, err := Load(path); err != nil {
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
