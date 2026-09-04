package config

import (
	"strings"
	"testing"

	"cubehaul/internal/version"
)

func TestEffectiveUserAgent(t *testing.T) {
	// No user_agent configured: the default carries the build version.
	got := (&Config{}).EffectiveUserAgent()
	want := "cubehaul/" + version.Value() + " (contact: set user_agent in ~/.cubehaul/config.json)"
	if got != want {
		t.Errorf("EffectiveUserAgent() = %q, want %q", got, want)
	}
	if !strings.Contains(got, version.Value()) {
		t.Errorf("default UA %q should embed version %q", got, version.Value())
	}

	// A configured user_agent wins verbatim, for API and downloads alike.
	cfg := &Config{UserAgent: "myname/1.0 (me@example.com)"}
	if got := cfg.EffectiveUserAgent(); got != cfg.UserAgent {
		t.Errorf("EffectiveUserAgent() = %q, want the configured %q", got, cfg.UserAgent)
	}
}
