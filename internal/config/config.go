// Package config handles loading of ModFetch configuration from
// environment variables and ~/.cubehaul/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cubehaul/internal/version"
)

// Config holds global settings.
type Config struct {
	CurseForgeAPIKey  string `json:"curseforge_api_key"`
	CurseForgeAPIBase string `json:"curseforge_api_base"`
	ModrinthAPIBase   string `json:"modrinth_api_base"`
	UserAgent         string `json:"user_agent"`
}

// DefaultUserAgent is used when no user_agent is configured. It carries the
// build version (see internal/version); Modrinth requires a User-Agent on
// every request.
func DefaultUserAgent() string {
	return "cubehaul/" + version.Value() + " (contact: set user_agent in ~/.cubehaul/config.json)"
}

// EffectiveUserAgent returns the configured user_agent, falling back to
// DefaultUserAgent when none is set. All HTTP traffic — API clients and
// file downloads — sends this.
func (c *Config) EffectiveUserAgent() string {
	if c.UserAgent != "" {
		return c.UserAgent
	}
	return DefaultUserAgent()
}

// Default API bases. Users can override these (see Load), e.g. to point at
// a compatible read-only mirror/cache exposing the same CurseForge v1 and
// Modrinth v2 JSON schemas.
const (
	DefaultCurseForgeBase = "https://api.curseforge.com/v1"
	DefaultModrinthBase   = "https://api.modrinth.com/v2"
)

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
	if v := os.Getenv("CURSEFORGE_API_BASE"); v != "" {
		cfg.CurseForgeAPIBase = v
	}
	if v := os.Getenv("MODRINTH_API_BASE"); v != "" {
		cfg.ModrinthAPIBase = v
	}

	// Fill in defaults and normalize trailing slashes so that callers can
	// append paths like "/mods/search" without worrying about double slashes.
	if cfg.CurseForgeAPIBase == "" {
		cfg.CurseForgeAPIBase = DefaultCurseForgeBase
	}
	if cfg.ModrinthAPIBase == "" {
		cfg.ModrinthAPIBase = DefaultModrinthBase
	}
	cfg.CurseForgeAPIBase = strings.TrimRight(cfg.CurseForgeAPIBase, "/")
	cfg.ModrinthAPIBase = strings.TrimRight(cfg.ModrinthAPIBase, "/")

	return cfg, nil
}

// ConfigPath returns the location of the config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory: %w", err)
	}
	return filepath.Join(home, ".cubehaul", "config.json"), nil
}
