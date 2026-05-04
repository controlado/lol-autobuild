package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/config"
)

func TestLoadEnvFileFromConfigNoPathNoOp(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnvFile.Path = ""

	if err := loadEnvFileFromConfig("config.example.yaml", cfg); err != nil {
		t.Fatalf("loadEnvFileFromConfig() error = %v", err)
	}
}

func TestLoadEnvFileFromConfigLoadsRelativePath(t *testing.T) {
	const key = "LOL_AUTOBUILD_TEST_RELATIVE_ENV_KEY"

	restore := preserveEnv(t, key)
	defer restore()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	envPath := filepath.Join(dir, ".env.local")

	if err := os.WriteFile(envPath, []byte(key+"=relative\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cfg := config.Defaults()
	cfg.EnvFile.Path = ".env.local"

	if err := loadEnvFileFromConfig(configPath, cfg); err != nil {
		t.Fatalf("loadEnvFileFromConfig() error = %v", err)
	}

	if got := os.Getenv(key); got != "relative" {
		t.Fatalf("unexpected env value: %q", got)
	}
}

func TestLoadEnvFileFromConfigLoadsAbsolutePath(t *testing.T) {
	const key = "LOL_AUTOBUILD_TEST_ABSOLUTE_ENV_KEY"

	restore := preserveEnv(t, key)
	defer restore()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	envPath := filepath.Join(dir, ".env.absolute")

	if err := os.WriteFile(envPath, []byte(key+"=absolute\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cfg := config.Defaults()
	cfg.EnvFile.Path = envPath

	if err := loadEnvFileFromConfig(configPath, cfg); err != nil {
		t.Fatalf("loadEnvFileFromConfig() error = %v", err)
	}

	if got := os.Getenv(key); got != "absolute" {
		t.Fatalf("unexpected env value: %q", got)
	}
}

func TestLoadEnvFileFromConfigReturnsErrorWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	missingPath := filepath.Join(dir, ".env.missing")

	cfg := config.Defaults()
	cfg.EnvFile.Path = ".env.missing"

	err := loadEnvFileFromConfig(configPath, cfg)
	if err == nil {
		t.Fatal("expected error for missing env file")
	}

	absMissingPath, absErr := filepath.Abs(missingPath)
	if absErr != nil {
		t.Fatalf("filepath.Abs() error = %v", absErr)
	}

	if !strings.Contains(err.Error(), absMissingPath) {
		t.Fatalf("expected error to include resolved path %q, got %q", absMissingPath, err.Error())
	}
}

func TestLoadEnvFileFromConfigDoesNotOverwriteExistingValue(t *testing.T) {
	const key = "LOL_AUTOBUILD_TEST_EXISTING_ENV_KEY"

	restore := preserveEnv(t, key)
	defer restore()

	if err := os.Setenv(key, "existing"); err != nil {
		t.Fatalf("set env: %v", err)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	envPath := filepath.Join(dir, ".env.local")

	if err := os.WriteFile(envPath, []byte(key+"=from-file\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cfg := config.Defaults()
	cfg.EnvFile.Path = ".env.local"

	if err := loadEnvFileFromConfig(configPath, cfg); err != nil {
		t.Fatalf("loadEnvFileFromConfig() error = %v", err)
	}

	if got := os.Getenv(key); got != "existing" {
		t.Fatalf("expected existing value to win, got %q", got)
	}
}

func preserveEnv(t *testing.T, key string) func() {
	t.Helper()

	previousValue, hadValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env: %v", err)
	}

	return func() {
		if hadValue {
			_ = os.Setenv(key, previousValue)
			return
		}

		_ = os.Unsetenv(key)
	}
}
