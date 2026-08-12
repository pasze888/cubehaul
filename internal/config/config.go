// Package config handles loading of ModFetch configuration from
// environment variables and ~/.modfetch/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds global settings.
type Config struct {
	CurseForgeAPIKey string `json:"curseforge_api_key"`
	UserAgent        string `json:"user_agent"`
}

// DefaultUserAgent is used when no User-Agent is configured.
// Modrinth requires a User-Agent on every request.
const DefaultUserAgent = "modfetch/0.1.0 (contact: set user_agent in ~/.modfetch/config.json)"

// Load reads configuration. Precedence: environment variable > config file.
func Load() (*Config, error) {
	cfg := &Config{}

	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if v := os.Getenv("CURSEFORGE_API_KEY"); v != "" {
		cfg.CurseForgeAPIKey = v
	}
	return cfg, nil
}

// ConfigPath returns the location of the config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory: %w", err)
	}
	return filepath.Join(home, ".modfetch", "config.json"), nil
}
