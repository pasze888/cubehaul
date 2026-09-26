package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Key describes one user-editable entry of the config file. The Keys table is
// the single source of truth behind "cubehaul config": listing, lookup,
// validation and the env/file/default precedence all derive from it.
type Key struct {
	// Name is the JSON key in the config file, e.g. "curseforge_api_key".
	Name string
	// Env is the environment variable overriding the file value ("" = none).
	Env string
	// Secret marks values that "config list" masks unless --show-secrets is set.
	Secret bool
	// Help is a one-line description, used in error messages and help text.
	Help string
	// Default returns the built-in value used when neither the environment nor
	// the file provides one; "" means the key has no default.
	Default func() string
	// Validate rejects values that could not be used at run time.
	Validate func(string) error
	// Normalize is applied to the effective value, mirroring what Load does
	// before handing a config to the clients (nil = identity).
	Normalize func(string) string
}

// Keys lists the editable configuration entries in display order. It mirrors
// the Config struct read by Load, which stays the request-time path.
var Keys = []Key{
	{
		Name:    "curseforge_api_key",
		Env:     "CURSEFORGE_API_KEY",
		Secret:  true,
		Help:    "CurseForge API key (get one at https://console.curseforge.com)",
		Default: noDefault,
		Validate: func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("must not be empty")
			}
			return nil
		},
	},
	{
		Name:      "curseforge_api_base",
		Env:       "CURSEFORGE_API_BASE",
		Help:      "base URL of the CurseForge v1 API",
		Default:   func() string { return DefaultCurseForgeBase },
		Validate:  validateBaseURL,
		Normalize: trimTrailingSlashes,
	},
	{
		Name:      "modrinth_api_base",
		Env:       "MODRINTH_API_BASE",
		Help:      "base URL of the Modrinth v2 API",
		Default:   func() string { return DefaultModrinthBase },
		Validate:  validateBaseURL,
		Normalize: trimTrailingSlashes,
	},
	{
		Name:     "user_agent",
		Help:     "User-Agent for the APIs and for downloads; put your contact address here",
		Default:  DefaultUserAgent,
		Validate: validateUserAgent,
	},
}

// noDefault marks a key with no built-in value.
func noDefault() string { return "" }

// LookupKey returns the Key named name.
func LookupKey(name string) (Key, bool) {
	for _, k := range Keys {
		if k.Name == name {
			return k, true
		}
	}
	return Key{}, false
}

// KeyNames returns the editable key names in table order.
func KeyNames() []string {
	names := make([]string, 0, len(Keys))
	for _, k := range Keys {
		names = append(names, k.Name)
	}
	return names
}

// Source records where an effective configuration value came from.
type Source string

const (
	SourceEnv     Source = "env"     // environment variable
	SourceFile    Source = "file"    // config file
	SourceDefault Source = "default" // built-in default
	SourceUnset   Source = "unset"   // nothing provides a value
)

// Value is one configuration key together with the value in effect for it.
type Value struct {
	Key    string
	Value  string
	Source Source
	Secret bool
}

// Resolve applies the documented precedence -- environment variable, then the
// config file, then the built-in default -- to k. file holds the raw config
// file contents as returned by LoadFile.
//
// An empty entry behaves like a missing one, which is how Load fills in
// defaults, so a value cleared by hand falls back instead of sticking around as
// an empty string.
func (k Key) Resolve(file map[string]json.RawMessage) (Value, error) {
	v := Value{Key: k.Name, Secret: k.Secret, Source: SourceUnset}

	switch env := os.Getenv(k.Env); {
	case k.Env != "" && env != "":
		v.Value, v.Source = env, SourceEnv
	default:
		if raw, ok := file[k.Name]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return Value{}, fmt.Errorf("config %s: value must be a string", k.Name)
			}
			if s != "" {
				v.Value, v.Source = s, SourceFile
			}
		}
	}

	if v.Source == SourceUnset && k.Default != nil {
		if def := k.Default(); def != "" {
			v.Value, v.Source = def, SourceDefault
		}
	}
	if k.Normalize != nil {
		v.Value = k.Normalize(v.Value)
	}
	return v, nil
}

// LoadFile reads the raw config file as a key -> JSON value map. Unknown keys are
// kept verbatim so that saving never drops entries written by hand or by a newer
// version. A missing or empty file yields an empty map.
func LoadFile(path string) (map[string]json.RawMessage, error) {
	vals := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return vals, nil
	case err != nil:
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return vals, nil
	}
	if err := json.Unmarshal(data, &vals); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if vals == nil { // the file held a literal null
		vals = map[string]json.RawMessage{}
	}
	return vals, nil
}

// SaveFile replaces the config file with vals, atomically and with 0600
// permissions: the file can hold an API key, and a half-written config would be
// far worse than a failed write. Windows ignores the mode bits. The parent
// directory is created when missing.
func SaveFile(path string, vals map[string]json.RawMessage) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(vals, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	// os.CreateTemp already creates the file with mode 0600.
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename below has succeeded

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// validateBaseURL accepts the http(s) base URLs handed to the API clients.
func validateBaseURL(v string) error {
	u, err := url.Parse(v)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("URL must include a host")
	}
	return nil
}

// validateUserAgent keeps the User-Agent sendable: both APIs and the CDN take it
// as an HTTP header, so an empty or multi-line value would fail at request time.
func validateUserAgent(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("must not be empty")
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return errors.New("must not contain control characters")
		}
	}
	return nil
}

// trimTrailingSlashes mirrors the normalization Load applies to API bases, so
// callers can append "/mods/search" without doubling the separator.
func trimTrailingSlashes(v string) string { return strings.TrimRight(v, "/") }
