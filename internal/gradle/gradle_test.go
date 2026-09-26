package gradle

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	src := "# a comment\n" +
		"! another comment\n" +
		"org.gradle.jvmargs=-Xmx3G\n" +
		"\n" +
		"minecraft_version = 1.20.1\n" +
		"mapping_channel: official\n" +
		"empty=\n" +
		"continued=1.21.\\\n" +
		"1\n" +
		"escaped=a\tb\n" +
		"later=1\n" +
		"later=2\n"

	props, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Properties{
		"org.gradle.jvmargs": "-Xmx3G",
		"minecraft_version":  "1.20.1",
		"mapping_channel":    "official",
		"empty":              "",
		"continued":          "1.21.1", // backslash continuation
		"escaped":            "a\tb",
		"later":              "2", // the last definition wins, as in the JDK
	}
	if !reflect.DeepEqual(props, want) {
		t.Errorf("Parse = %#v, want %#v", props, want)
	}

	// Windows line endings and a missing trailing newline are fine.
	props, err = Parse(strings.NewReader("a=1\r\nb=2"))
	if err != nil {
		t.Fatalf("Parse(CRLF): %v", err)
	}
	if props["a"] != "1" || props["b"] != "2" {
		t.Errorf("Parse(CRLF) = %#v", props)
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "targets", "neoforge-1.21.1")
	deep := filepath.Join(target, "src", "main", "java")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(target, FileName)
	if err := os.WriteFile(file, []byte("minecraft_version=1.21.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Find(deep)
	if err != nil {
		t.Fatalf("Find(deep): %v", err)
	}
	if got != file {
		t.Errorf("Find(deep) = %s, want the nearest file %s", got, file)
	}

	// The file itself is accepted, so --gradle=gradle.properties works.
	if got, err := Find(file); err != nil || got != file {
		t.Errorf("Find(file) = %s, %v", got, err)
	}

	// Anything else is a user error rather than a silent walk upwards.
	other := filepath.Join(root, "build.gradle")
	if err := os.WriteFile(other, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(other); err == nil {
		t.Error("want an error for a file that is not gradle.properties")
	}
	if _, err := Find(filepath.Join(root, "nope", FileName)); err == nil {
		t.Error("want an error for a path that does not exist")
	}
}

func TestLoadReadsTheNearestFile(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "src", "main")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "minecraft_version=1.21.1\nneo_version=21.1.244\n"
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(deep)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.File != filepath.Join(root, FileName) {
		t.Errorf("File = %s", got.File)
	}
	if got.MinecraftVersion != "1.21.1" || got.MinecraftVersionSource != "minecraft_version" {
		t.Errorf("version = %q from %q", got.MinecraftVersion, got.MinecraftVersionSource)
	}
	if !reflect.DeepEqual(got.Loaders, []string{LoaderNeoForge}) {
		t.Errorf("Loaders = %v", got.Loaders)
	}
}

func TestLoadFallsBackToAnAncestor(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// The module's own file only carries mod metadata ...
	if err := os.WriteFile(filepath.Join(root, "sub", FileName), []byte("mod_version=1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// ... while the multi-module root declares the target.
	rootProps := "minecraft_version=1.21.1\nneo_version=21.1.244\n"
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(rootProps), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(sub)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.File != filepath.Join(root, FileName) {
		t.Errorf("File = %s, want the ancestor that declares a target", got.File)
	}
	if got.MinecraftVersion != "1.21.1" || !reflect.DeepEqual(got.Loaders, []string{LoaderNeoForge}) {
		t.Errorf("Load = %+v", got)
	}

	// Find still reports the nearest file, which is what --gradle=<file> and
	// "target" use to stay predictable.
	if nearest, err := Find(sub); err != nil || nearest != filepath.Join(root, "sub", FileName) {
		t.Errorf("Find = %s, %v; want the nearest file", nearest, err)
	}
}

// TestDetect covers the property shapes found in real MDKs, including the ones
// that are easy to get wrong: prefixed multi-target keys, range-only projects
// and keys that look like loader evidence but are not.
func TestDetect(t *testing.T) {
	cases := []struct {
		name        string
		props       Properties
		wantVersion string
		wantSource  string
		wantLoaders []string
	}{
		{
			name: "fabric MDK",
			props: Properties{
				"minecraft_version": "1.20.1",
				"yarn_mappings":     "1.20.1+build.10",
				"loader_version":    "0.16.10",
				"fabric_version":    "0.92.3+1.20.1",
				"mod_version":       "1.0.0",
			},
			wantVersion: "1.20.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderFabric},
		},
		{
			name: "fabric with only a bare loader_version",
			props: Properties{
				"minecraft_version": "1.20.1",
				"loader_version":    "0.16.10",
			},
			wantVersion: "1.20.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderFabric},
		},
		{
			name: "forge MDK",
			props: Properties{
				"minecraft_version":       "1.20.1",
				"minecraft_version_range": "[1.20.1,1.21)",
				"forge_version":           "47.4.10",
				"forge_version_range":     "[47,)",
				"loader_version_range":    "[47,)",
			},
			wantVersion: "1.20.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderForge},
		},
		{
			name: "neoforge MDK with camelCase keys",
			props: Properties{
				"minecraftVersion": "1.21.1",
				"neoforgeVersion":  "21.1.172",
			},
			wantVersion: "1.21.1", wantSource: "minecraftVersion",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "neo_version",
			props: Properties{
				"minecraft_version": "1.21.1",
				"neo_version":       "21.1.244",
			},
			wantVersion: "1.21.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "multi-target project",
			props: Properties{
				"neoforge_121_minecraft_version": "1.21.1",
				"neoforge_121_version":           "21.1.234",
				"neoforge_121_version_range":     "[21.1,)",
			},
			wantVersion: "1.21.1", wantSource: "neoforge_121_minecraft_version",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "range only",
			props: Properties{
				"minecraft_version_range": "[1.21,1.21.1)",
				"neo_version_range":       "[21.0.0-beta,)",
				"loader_version_range":    "[4,)",
				"mod_version":             "1.6.0",
			},
			wantVersion: "1.21", wantSource: "minecraft_version_range",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "explicit mod_loader",
			props: Properties{
				"minecraft_version": "1.21.1",
				"neo_version":       "21.1.244",
				"mod_loader":        "neoforge",
			},
			wantVersion: "1.21.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "javafml describes the language, not the loader",
			props: Properties{
				"minecraft_version": "1.21.1",
				"neo_version":       "21.1.244",
				"modLoader":         "javafml",
			},
			wantVersion: "1.21.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "architectury builds for two loaders",
			props: Properties{
				"minecraft_version": "1.20.1",
				"fabric_version":    "0.92.3+1.20.1",
				"forge_version":     "47.3.0",
			},
			wantVersion: "1.20.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderFabric, LoaderForge},
		},
		{
			name: "quilt",
			props: Properties{
				"minecraft_version":    "1.20.1",
				"quilt_loader_version": "0.23.1",
			},
			wantVersion: "1.20.1", wantSource: "minecraft_version",
			wantLoaders: []string{LoaderQuilt},
		},
		{
			name: "dependency Minecraft versions are not the target",
			props: Properties{
				"neoForge.parchment.minecraftVersion": "1.21.9",
				"jeiMinecraftVersion":                 "26.1.2",
				"emiMinecraftVersion":                 "1.21.1",
			},
			wantVersion: "", wantSource: "",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "toolchain-prefixed keys are the target",
			props: Properties{
				"forge_1201_minecraft_version": "1.20.1",
				"forge_1201_version":           "47.4.10",
			},
			wantVersion: "1.20.1", wantSource: "forge_1201_minecraft_version",
			wantLoaders: []string{LoaderForge},
		},
		{
			name: "explicit modLoaders outranks a misleading forgeVersion",
			props: Properties{
				"mcVersion":    "1.21.1",
				"forgeVersion": "21.1.187",
				"modLoaders":   "NeoForge",
				"jeiVersion":   "19.21.2.313",
			},
			wantVersion: "1.21.1", wantSource: "mcVersion",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "dotted mapping key is not a target",
			props: Properties{
				"neoforgeVersion":                     "26.1.2.87",
				"neoForge.parchment.minecraftVersion": "1.21.9",
				"ae2Version":                          "26.1.10-beta",
			},
			wantVersion: "", wantSource: "",
			wantLoaders: []string{LoaderNeoForge},
		},
		{
			name: "nothing to detect",
			props: Properties{
				"mod_version":             "1.3.0",
				"java_version":            "21",
				"refinedarchitectVersion": "1.7.0",
			},
			wantVersion: "", wantSource: "",
			wantLoaders: []string{},
		},
	}

	for _, c := range cases {
		version, source, loaders := Detect(c.props)
		if version != c.wantVersion || source != c.wantSource {
			t.Errorf("%s: version = %q from %q, want %q from %q", c.name, version, source, c.wantVersion, c.wantSource)
		}
		if !reflect.DeepEqual(loaders, c.wantLoaders) {
			t.Errorf("%s: loaders = %v, want %v", c.name, loaders, c.wantLoaders)
		}
	}
}
