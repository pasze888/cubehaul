package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestKeysAreWellFormed(t *testing.T) {
	if len(Keys) == 0 {
		t.Fatal("Keys is empty")
	}
	for _, k := range Keys {
		if k.Name == "" || k.Help == "" {
			t.Errorf("key %+v needs both a name and help text", k)
		}
		if k.Default == nil {
			t.Errorf("key %s: Default must be set (return \"\" when there is none)", k.Name)
		}
		if _, ok := LookupKey(k.Name); !ok {
			t.Errorf("LookupKey(%q) did not find the key", k.Name)
		}
	}
	if _, ok := LookupKey("nope"); ok {
		t.Error("LookupKey should reject an unknown name")
	}
	if got := len(KeyNames()); got != len(Keys) {
		t.Errorf("KeyNames() returned %d names, want %d", got, len(Keys))
	}
}

// clearConfigEnv keeps the tests independent of the developer's environment.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range Keys {
		if k.Env != "" {
			t.Setenv(k.Env, "")
		}
	}
}

func TestResolvePrecedence(t *testing.T) {
	clearConfigEnv(t)
	key, ok := LookupKey("curseforge_api_base")
	if !ok {
		t.Fatal("curseforge_api_base is missing from Keys")
	}
	file := map[string]json.RawMessage{
		"curseforge_api_base": json.RawMessage(`"https://mirror.example/v1/"`),
	}

	// The file wins over the default, and is normalized the way Load does it.
	v, err := key.Resolve(file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Value != "https://mirror.example/v1" || v.Source != SourceFile {
		t.Errorf("Resolve(file) = %q/%q, want the normalized file value", v.Value, v.Source)
	}

	// The environment wins over the file.
	t.Setenv("CURSEFORGE_API_BASE", "http://127.0.0.1:8080")
	v, err = key.Resolve(file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Value != "http://127.0.0.1:8080" || v.Source != SourceEnv {
		t.Errorf("Resolve(env+file) = %q/%q, want the environment value", v.Value, v.Source)
	}
}

func TestResolveDefaultsAndUnset(t *testing.T) {
	clearConfigEnv(t)

	base, _ := LookupKey("modrinth_api_base")
	v, err := base.Resolve(map[string]json.RawMessage{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Value != DefaultModrinthBase || v.Source != SourceDefault {
		t.Errorf("Resolve(default) = %q/%q, want the built-in base", v.Value, v.Source)
	}

	// A key without a default is reported as unset rather than as an empty default.
	key, _ := LookupKey("curseforge_api_key")
	v, err = key.Resolve(map[string]json.RawMessage{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Value != "" || v.Source != SourceUnset {
		t.Errorf("Resolve(nothing) = %q/%q, want unset", v.Value, v.Source)
	}

	// An empty entry behaves like a missing one, exactly as Load fills defaults.
	ua, _ := LookupKey("user_agent")
	if ua.Secret {
		t.Error("user_agent must not be hidden as a secret")
	}
	v, err = ua.Resolve(map[string]json.RawMessage{"user_agent": json.RawMessage(`""`)})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Value != DefaultUserAgent() || v.Source != SourceDefault {
		t.Errorf("Resolve(empty file value) = %q/%q, want the default", v.Value, v.Source)
	}
}

func TestResolveRejectsNonStringValue(t *testing.T) {
	clearConfigEnv(t)
	key, _ := LookupKey("user_agent")
	_, err := key.Resolve(map[string]json.RawMessage{"user_agent": json.RawMessage(`42`)})
	if err == nil {
		t.Fatal("want an error for a non-string config value")
	}
	if !strings.Contains(err.Error(), "user_agent") {
		t.Errorf("error %q should name the offending key", err)
	}
}

func TestKeyValidation(t *testing.T) {
	cases := []struct {
		key     string
		value   string
		wantErr bool
	}{
		{"curseforge_api_key", "$2a$10$abcdef", false},
		{"curseforge_api_key", "", true},
		{"curseforge_api_key", "   ", true},
		{"curseforge_api_base", "https://api.curseforge.com/v1", false},
		{"curseforge_api_base", "http://127.0.0.1:8080", false},
		{"curseforge_api_base", "api.curseforge.com", true},
		{"curseforge_api_base", "ftp://mirror.example/", true},
		{"curseforge_api_base", "https://", true},
		{"modrinth_api_base", "https://api.modrinth.com/v2", false},
		{"user_agent", "myname/1.0 (me@example.com)", false},
		{"user_agent", "", true},
		{"user_agent", "two\nlines", true},
	}
	for _, c := range cases {
		k, ok := LookupKey(c.key)
		if !ok {
			t.Fatalf("%s is missing from Keys", c.key)
		}
		err := k.Validate(c.value)
		if (err != nil) != c.wantErr {
			t.Errorf("Validate(%s, %q) = %v, wantErr %v", c.key, c.value, err, c.wantErr)
		}
	}
}

func TestLoadFileMissingEmptyAndBroken(t *testing.T) {
	dir := t.TempDir()

	vals, err := LoadFile(filepath.Join(dir, "missing.json"))
	if err != nil || len(vals) != 0 {
		t.Fatalf("LoadFile(missing) = %v, %v; want an empty map", vals, err)
	}

	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// A missing, empty, blank or null file all read as "nothing configured".
	for _, c := range []struct{ name, content string }{
		{"empty.json", ""},
		{"blank.json", "\n \t\n"},
		{"null.json", "null"},
	} {
		if vals, err := LoadFile(write(c.name, c.content)); err != nil || len(vals) != 0 {
			t.Errorf("LoadFile(%s) = %v, %v; want an empty map", c.name, vals, err)
		}
	}

	// Anything that is not a JSON object is a hard error: silently ignoring it
	// would hide a config the user believes is in effect.
	for _, c := range []struct{ name, content string }{
		{"broken.json", "{not json"},
		{"array.json", "[]"},
	} {
		if _, err := LoadFile(write(c.name, c.content)); err == nil {
			t.Errorf("LoadFile(%s) should fail: the file is not a JSON object", c.name)
		}
	}
}

func TestSaveFileReplacesAndPreserves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	// Saving creates the parent directory.
	orig := map[string]json.RawMessage{
		"user_agent": json.RawMessage(`"myname/1.0"`),
		"future_key": json.RawMessage(`{"nested": true}`),
	}
	if err := SaveFile(path, orig); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if string(got["user_agent"]) != `"myname/1.0"` {
		t.Errorf("user_agent = %s, want the saved string", got["user_agent"])
	}
	future, ok := got["future_key"]
	if !ok {
		t.Fatal("keys written by hand or by a newer version must survive a save")
	}
	var want, have any
	if err := json.Unmarshal(orig["future_key"], &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(future, &have); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, have) {
		t.Errorf("future_key = %s, want %s", future, orig["future_key"])
	}

	// Saving again replaces the file rather than appending to it.
	if err := SaveFile(path, map[string]json.RawMessage{"user_agent": json.RawMessage(`"second/1.0"`)}); err != nil {
		t.Fatalf("SaveFile (replace): %v", err)
	}
	got, err = LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if string(got["user_agent"]) != `"second/1.0"` {
		t.Errorf("user_agent = %s after replacing", got["user_agent"])
	}
	if _, ok := got["future_key"]; ok {
		t.Error("the replaced file should hold only what was saved")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("config file mode = %o, want 600", perm)
		}
	}
}
