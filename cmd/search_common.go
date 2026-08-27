package cmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"cubehaul/internal/platform"
)

// searchCommonFlags holds the search inputs shared by both platform
// sub-commands (cubehaul modrinth search / cubehaul curseforge search).
type searchCommonFlags struct {
	projectType  string
	categories   []string
	loaders      []string
	gameVersions []string
	sort         string
	limit        int
	offset       int
}

// addSearchCommonFlags registers the platform-agnostic search flags on f.
func addSearchCommonFlags(f *pflag.FlagSet, s *searchCommonFlags) {
	f.StringVar(&s.projectType, "project-type", "", "project type: mod, modpack, resourcepack, shader, plugin, datapack, ... (curseforge maps this to a class id)")
	f.StringSliceVar(&s.categories, "category", nil, "category name/slug, repeatable, ORed together")
	f.StringSliceVar(&s.loaders, "loader", nil, "mod loader, repeatable (fabric/forge/neoforge/quilt/...)")
	f.StringSliceVar(&s.gameVersions, "game-version", nil, "Minecraft version, e.g. 1.20.1 (repeatable; curseforge uses only the first)")
	f.StringVar(&s.sort, "sort", "", "sort order (see each platform sub-command for supported values)")
	f.IntVar(&s.limit, "limit", 10, "max results (curseforge caps at 50)")
	f.IntVar(&s.offset, "offset", 0, "results to skip (curseforge: index)")
}

// toSearchOptions converts the shared flags into the common portion of a
// SearchOptions. Platform-specific fields are filled in by the caller.
func (s *searchCommonFlags) toSearchOptions(plat, query string) platform.SearchOptions {
	return platform.SearchOptions{
		Query:        query,
		PlatformName: plat,
		ProjectType:  s.projectType,
		Categories:   s.categories,
		Loaders:      s.loaders,
		GameVersions: s.gameVersions,
		Sort:         s.sort,
		Limit:        s.limit,
		Offset:       s.offset,
	}
}

// renderFlagSubset renders the flags selected by fn using pflag's standard
// alignment (shared *pflag.Flag pointers, so usage text stays in sync with
// the live flag set).
func renderFlagSubset(fs *pflag.FlagSet, fn func(name string) bool) string {
	sub := pflag.NewFlagSet("", pflag.ContinueOnError)
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || !fn(f.Name) {
			return
		}
		sub.AddFlag(f)
	})
	return sub.FlagUsages()
}

// writeGroupedUsage renders a help body with flags split into titled groups.
func writeGroupedUsage(c *cobra.Command, groups []struct {
	title string
	flags []string
}) error {
	var b strings.Builder
	b.WriteString("Usage:\n  ")
	b.WriteString(c.UseLine())
	b.WriteString("\n")
	if c.HasExample() {
		b.WriteString("\nExamples:\n")
		b.WriteString(c.Example)
		b.WriteString("\n")
	}

	if c.HasAvailableLocalFlags() {
		b.WriteString("\nFlags:\n")
		inAnyGroup := func(name string) bool {
			for _, g := range groups {
				for _, f := range g.flags {
					if f == name {
						return true
					}
				}
			}
			return false
		}
		b.WriteString(renderFlagSubset(c.LocalFlags(), func(name string) bool {
			return !inAnyGroup(name)
		}))
		for _, g := range groups {
			in := func(name string) bool {
				for _, f := range g.flags {
					if f == name {
						return true
					}
				}
				return false
			}
			if s := renderFlagSubset(c.LocalFlags(), in); s != "" {
				b.WriteString("\n")
				b.WriteString(g.title)
				b.WriteString(":\n")
				b.WriteString(s)
			}
		}
	}
	if c.HasAvailableInheritedFlags() {
		b.WriteString("\nGlobal Flags:\n")
		b.WriteString(c.InheritedFlags().FlagUsages())
	}
	_, err := io.WriteString(c.OutOrStderr(), b.String())
	return err
}
