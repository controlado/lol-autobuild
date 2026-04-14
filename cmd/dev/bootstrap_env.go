package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"

	"github.com/controlado/lol-autobuild/internal/config"
)

func loadEnvFileFromConfig(configPath string, cfg config.Config) error {
	envPath := strings.TrimSpace(cfg.EnvFile.Path)
	if envPath == "" {
		return nil
	}

	resolvedPath, err := resolveEnvFilePath(configPath, envPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("env file not found: %s", resolvedPath)
		}

		return fmt.Errorf("stat env file %s: %w", resolvedPath, err)
	}

	values, err := godotenv.Read(resolvedPath)
	if err != nil {
		return fmt.Errorf("load env file %s: %w", resolvedPath, err)
	}

	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env var %q from %s: %w", key, resolvedPath, err)
		}
	}

	return nil
}

func resolveEnvFilePath(configPath string, envPath string) (string, error) {
	resolvedPath := envPath
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(filepath.Dir(configPath), resolvedPath)
	}

	resolvedPath = filepath.Clean(resolvedPath)
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("resolve env file path %q: %w", envPath, err)
	}

	return absPath, nil
}
