// Package gradle reads the Minecraft version and mod loader a mod project
// targets from its gradle.properties.
//
// The property names come from real Mod Development Kits rather than from a
// specification: Fabric Loom, ForgeGradle and NeoGradle disagree on nearly
// everything, multi-target repositories prefix the keys per target
// ("neoforge_121_minecraft_version"), and some projects only publish a Maven
// version range. Detection therefore normalizes every key and scans the whole
// file instead of looking up a fixed list.
package gradle

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FileName is the file this package looks for.
const FileName = "gradle.properties"

// Loader names, spelled the way the APIs and --loader spell them.
const (
	LoaderFabric   = "fabric"
	LoaderForge    = "forge"
	LoaderNeoForge = "neoforge"
	LoaderQuilt    = "quilt"
)

// Properties is a Java-properties key/value map. As in the JDK implementation a
// later definition replaces an earlier one.
type Properties map[string]string

// Target is what a gradle.properties says about the project it belongs to.
type Target struct {
	// File is the gradle.properties the values were read from.
	File string
	// MinecraftVersion is empty when the file does not pin one.
	MinecraftVersion string
	// MinecraftVersionSource names the property MinecraftVersion came from, so a
	// value derived from a version range is visible as such.
	MinecraftVersionSource string
	// Loaders lists the detected mod loaders, sorted; empty when none was found
	// and longer than one when the project builds for several.
	Loaders []string
}

// Parse reads the subset of the Java properties format that MDK files use:
// "key=value" and "key: value" pairs, "#" and "!" comments, backslash line
// continuations and the common escapes. Values keep their literal text otherwise.
func Parse(r io.Reader) (Properties, error) {
	props := Properties{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var logical strings.Builder
	continuing := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if continuing {
			line = strings.TrimLeft(line, " \t\f")
		} else {
			line = strings.TrimLeft(line, " \t\f")
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
		}
		if hasContinuation(line) {
			logical.WriteString(strings.TrimSuffix(line, "\\"))
			continuing = true
			continue
		}
		logical.WriteString(line)
		key, value := splitProperty(logical.String())
		logical.Reset()
		continuing = false
		if key != "" {
			props[key] = value
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return props, nil
}

// hasContinuation reports whether the line ends with an odd number of
// backslashes, which carries the logical line over to the next one.
func hasContinuation(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// splitProperty splits a logical line into key and value at the first unescaped
// "=" or ":". A line without a separator is a key with an empty value.
func splitProperty(line string) (string, string) {
	sep := -1
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			i++ // skip the escaped character, which cannot be a separator
			continue
		}
		if line[i] == '=' || line[i] == ':' {
			sep = i
			break
		}
	}
	if sep < 0 {
		return strings.TrimSpace(line), ""
	}
	return strings.TrimSpace(line[:sep]), unescape(strings.TrimSpace(line[sep+1:]))
}

// unescape resolves the escapes Java properties allow in values. Anything the
// JDK treats leniently keeps its literal character here too.
func unescape(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(s) {
				if code, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					r := rune(code)
					b.WriteRune(r)
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// normalize collapses the separator and case variants MDK authors use, so that
// "minecraft_version", "minecraftVersion" and "minecraft.version" all compare
// equal.
func normalize(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range strings.ToLower(key) {
		switch r {
		case '_', '-', '.', ' ':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Find returns the nearest gradle.properties at or above start. start may be a
// directory or the file itself.
func Find(start string) (string, error) {
	found, err := candidates(start)
	if err != nil {
		return "", err
	}
	return found[0], nil
}

// candidates lists the gradle.properties files at or above start, nearest first.
// An explicit file path is taken literally and never walks upwards.
func candidates(start string) ([]string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	switch {
	case err == nil && !info.IsDir():
		if filepath.Base(abs) != FileName {
			return nil, fmt.Errorf("%s is not a %s file", abs, FileName)
		}
		return []string{abs}, nil
	case errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("%s does not exist", abs)
	case err != nil:
		return nil, err
	}

	var found []string
	for dir := abs; ; {
		candidate := filepath.Join(dir, FileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no %s found in %s or any parent directory", FileName, abs)
	}
	return found, nil
}

// Load finds, parses and inspects the gradle.properties at or above start.
//
// The nearest file wins, but a project's own file often holds only mod metadata
// while the Minecraft version and loader live in the multi-module root's file
// (confluence/ConfluenceOtherworld next to confluence/gradle.properties). So when
// a file declares neither, the search continues upwards and the first file that
// declares something is used; Target.File always names the file the answer came
// from.
func Load(start string) (*Target, error) {
	paths, err := candidates(start)
	if err != nil {
		return nil, err
	}

	var nearest *Target
	for _, path := range paths {
		target, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		if nearest == nil {
			nearest = target
		}
		if target.MinecraftVersion != "" || len(target.Loaders) > 0 {
			return target, nil
		}
	}
	return nearest, nil
}

// loadFile parses one gradle.properties and inspects it.
func loadFile(path string) (*Target, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	props, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	version, source, loaders := Detect(props)
	return &Target{
		File:                   path,
		MinecraftVersion:       version,
		MinecraftVersionSource: source,
		Loaders:                loaders,
	}, nil
}

// Detect extracts the Minecraft version and the mod loaders from parsed
// properties. The second return value names the property the version came from.
func Detect(props Properties) (version, source string, loaders []string) {
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	version, source = detectMinecraft(keys, props)
	return version, source, detectLoaders(keys, props)
}

// detectMinecraft looks for the version in three descending orders of
// confidence: an exact key, a target-prefixed key, then the lower bound of a
// version range. Dotted keys ("neoForge.parchment.minecraftVersion") are
// deliberately ignored: they describe a mapping configuration, not the project's
// target, and a wrong Minecraft version is worse than none.
func detectMinecraft(keys []string, props Properties) (string, string) {
	var exact, prefixed, ranged, exactKey, prefixedKey, rangedKey string
	for _, key := range keys {
		value := strings.TrimSpace(props[key])
		if value == "" {
			continue
		}
		norm := normalize(key)
		switch {
		case norm == "minecraftversion" || norm == "mcversion":
			if exact == "" {
				exact, exactKey = value, key
			}
		case strings.HasSuffix(norm, "minecraftversionrange") || strings.HasSuffix(norm, "mcversionrange"):
			if ranged == "" {
				if lower := rangeLowerBound(value); lower != "" {
					ranged, rangedKey = lower, key
				}
			}
		case (strings.HasSuffix(norm, "minecraftversion") || strings.HasSuffix(norm, "mcversion")) && !strings.Contains(key, "."):
			if prefixed == "" && targetScoped(norm) {
				prefixed, prefixedKey = value, key
			}
		}
	}
	switch {
	case exact != "":
		return exact, exactKey
	case prefixed != "":
		return prefixed, prefixedKey
	case ranged != "":
		return ranged, rangedKey
	}
	return "", ""
}

// targetScoped reports whether a prefixed "...minecraftVersion" key describes the
// project's own target rather than a dependency's Minecraft version:
// "neoforge_121_minecraft_version" is the target, while "emiMinecraftVersion" is
// the Minecraft version of EMI. Only toolchain and target prefixes count, so a
// dependency's property never becomes the project's filter.
func targetScoped(norm string) bool {
	prefix := strings.TrimSuffix(norm, "minecraftversion")
	if prefix == norm {
		prefix = strings.TrimSuffix(norm, "mcversion")
	}
	for _, token := range []string{"neoforge", "neo", "forge", "fabric", "quilt", "loom", "yarn", "target", "minecraft", "mc"} {
		if strings.HasPrefix(prefix, token) {
			return true
		}
	}
	return false
}

// rangeLowerBound pulls the first version out of a Maven-style range such as
// "[1.21,1.21.1)" or "[1.20.1,)".
func rangeLowerBound(value string) string {
	s := strings.TrimLeft(strings.TrimSpace(value), "[(")
	if i := strings.IndexAny(s, ",])"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" || s[0] < '0' || s[0] > '9' {
		return ""
	}
	return s
}

// detectLoaders treats property names as evidence: MDKs name their toolchain
// version after the loader ("neo_version", "neoforgeVersion", "fabric_version"),
// and a version range counts too, because only one toolchain defines it. A bare
// loader_version is weak evidence -- Fabric Loom means the loader by it, but
// ForgeGradle spells its FML range loader_version_range -- so it is only used
// when nothing else was found.
//
// A mod_loader / modLoaders declaration outranks all of that: it is the author
// stating the answer, and it is what keeps projects that call NeoForge's
// toolchain version "forgeVersion" from being reported as Forge.
func detectLoaders(keys []string, props Properties) []string {
	found := map[string]bool{}
	declared := map[string]bool{}
	weakFabric := false

	for _, key := range keys {
		norm := normalize(key)
		switch norm {
		case "modloader", "modloaders":
			for _, token := range strings.FieldsFunc(props[key], isLoaderSeparator) {
				if loader := loaderName(token); loader != "" {
					declared[loader] = true
				}
			}
			continue
		}
		if !strings.Contains(norm, "version") {
			continue
		}
		switch {
		case strings.HasPrefix(norm, "neoforge") || strings.HasPrefix(norm, "neo"):
			found[LoaderNeoForge] = true
		case strings.HasPrefix(norm, "quilt"):
			found[LoaderQuilt] = true
		case strings.HasPrefix(norm, "fabric"), strings.HasPrefix(norm, "yarn"), strings.HasPrefix(norm, "loom"):
			found[LoaderFabric] = true
		case strings.Contains(norm, "forge"):
			found[LoaderForge] = true
		case norm == "loaderversion":
			weakFabric = true
		}
	}

	if len(declared) > 0 {
		return sortedLoaders(declared)
	}
	if len(found) == 0 && weakFabric {
		found[LoaderFabric] = true
	}
	return sortedLoaders(found)
}

// isLoaderSeparator splits a modLoaders value such as "fabric,forge".
func isLoaderSeparator(r rune) bool {
	return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\t'
}

// sortedLoaders returns the loader names in a stable order. It returns an empty
// slice rather than nil so JSON output reads "loaders": [].
func sortedLoaders(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for loader := range set {
		out = append(out, loader)
	}
	sort.Strings(out)
	return out
}

// loaderName maps a mod_loader value onto a loader name. Values such as
// "javafml" describe the mod's language rather than its platform, and are
// ignored.
func loaderName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LoaderFabric:
		return LoaderFabric
	case LoaderForge:
		return LoaderForge
	case LoaderNeoForge, "neo":
		return LoaderNeoForge
	case LoaderQuilt:
		return LoaderQuilt
	default:
		return ""
	}
}
