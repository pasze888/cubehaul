package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

var searchFlags struct {
	platform          string
	projectType       string
	categories        []string
	loaders           []string
	gameVersions      []string
	openSource        bool
	noOpenSource      bool
	environment       string
	author            string
	license           string
	sort              string
	limit             int
	offset            int
	facets            []string
	facetsJSON        string
	sortOrder         string
	classID           int
	categoryID        int
	modID             int
	slug              string
	gameVersionTypeID int
	rawParams         []string
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search projects on Modrinth or CurseForge",
	Long: `Search projects on Modrinth or CurseForge.

Quick start: add --platform (required), --loader and --game-version and
run it; facets below are optional fine-grained filters.

The query is optional: omitting it lists projects filtered by the given flags.

Facets (Modrinth) are the precise filter system. --category, --loader and
--game-version are convenience flags that expand to facets; --facet passes
raw facet strings ("downloads<=100", "versions!=1.20.1") and --facets-json
passes the raw JSON array of arrays, e.g. [["categories:forge"],["versions:1.17.1"]].

CurseForge has no facet system; --raw-param passes arbitrary query parameters
through verbatim, e.g. --raw-param 'gameVersion=1.20.1'.`,
	Example: `  cubehaul search sodium --platform modrinth --loader fabric --limit 5
  cubehaul search sodium --platform curseforge --loader forge
  cubehaul search "" --platform modrinth --category adventure --sort downloads
  cubehaul search "" --platform modrinth --facet 'downloads>=100000000' --sort downloads`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSearch,
}

// searchFlagGroups splits search's flags into help sections.
// cobra has no display grouping for flags, so the usage output is rendered
// by searchUsageFunc.
var searchFlagGroups = []struct {
	title string
	flags []string
}{
	{"Common", []string{
		"platform", "project-type", "category", "loader",
		"game-version", "sort", "limit", "offset",
	}},
	{"Modrinth only", []string{
		"open-source", "no-open-source", "environment",
		"author", "license", "facet", "facets-json",
	}},
	{"CurseForge only", []string{
		"sort-order", "class-id", "category-id", "mod-id",
		"slug", "game-version-type-id", "raw-param",
	}},
}

// searchUsageFunc renders usage with flags split into groups.
func searchUsageFunc(c *cobra.Command) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage:\n  %s\n", c.UseLine())
	if c.HasExample() {
		fmt.Fprintf(&b, "\nExamples:\n%s\n", c.Example)
	}

	if c.HasAvailableLocalFlags() {
		b.WriteString("\nFlags:\n")
		// flags not assigned to any group (e.g. -h, --help)
		b.WriteString(renderFlagSubset(c.LocalFlags(), func(name string) bool {
			return !flagInAnyGroup(name)
		}))
		for _, g := range searchFlagGroups {
			in := func(name string) bool {
				for _, f := range g.flags {
					if f == name {
						return true
					}
				}
				return false
			}
			if s := renderFlagSubset(c.LocalFlags(), in); s != "" {
				fmt.Fprintf(&b, "\n%s:\n%s", g.title, s)
			}
		}
	}
	if c.HasAvailableInheritedFlags() {
		fmt.Fprintf(&b, "\nGlobal Flags:\n%s", c.InheritedFlags().FlagUsages())
	}
	_, err := io.WriteString(c.OutOrStderr(), b.String())
	return err
}

func flagInAnyGroup(name string) bool {
	for _, g := range searchFlagGroups {
		for _, f := range g.flags {
			if f == name {
				return true
			}
		}
	}
	return false
}

// renderFlagSubset renders the flags selected by fn using pflag's standard
// alignment (shared *pflag.Flag pointers, so usage text stays in sync).
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

func init() {
	f := searchCmd.Flags()
	f.StringVar(&searchFlags.platform, "platform", "", "platform to search: modrinth or curseforge (required)")
	f.StringVar(&searchFlags.projectType, "project-type", "", "project type: mod, modpack, resourcepack, shader, plugin, datapack, ... (curseforge maps this to a class id)")
	f.StringSliceVar(&searchFlags.categories, "category", nil, "category name/slug, repeatable, ORed together")
	f.StringSliceVar(&searchFlags.loaders, "loader", nil, "mod loader: fabric, forge, neoforge, quilt, ... (repeatable)")
	f.StringSliceVar(&searchFlags.gameVersions, "game-version", nil, "Minecraft version, e.g. 1.20.1 (repeatable; curseforge uses only the first)")
	f.BoolVar(&searchFlags.openSource, "open-source", false, "only open-source projects")
	f.BoolVar(&searchFlags.noOpenSource, "no-open-source", false, "only closed-source projects")
	f.StringVar(&searchFlags.environment, "environment", "", "supported environment: client, server, client_and_server")
	f.StringVar(&searchFlags.author, "author", "", "filter by author username")
	f.StringVar(&searchFlags.license, "license", "", "filter by SPDX license id, e.g. mit")
	f.StringVar(&searchFlags.sort, "sort", "", "sort: relevance|downloads|follows|newest|updated (modrinth), featured|popularity|updated|name|author|downloads (curseforge)")
	f.IntVar(&searchFlags.limit, "limit", 10, "max results (curseforge caps at 50)")
	f.IntVar(&searchFlags.offset, "offset", 0, "results to skip (curseforge: index)")
	f.StringSliceVar(&searchFlags.facets, "facet", nil, "raw modrinth facet, e.g. 'downloads<=100' or 'versions!=1.20.1' (repeatable)")
	f.StringVar(&searchFlags.facetsJSON, "facets-json", "", "raw modrinth facets as JSON array of arrays, e.g. '[[\"categories:forge\"],[\"versions:1.17.1\"]]'")
	f.StringVar(&searchFlags.sortOrder, "sort-order", "", "sort direction: asc or desc")
	f.IntVar(&searchFlags.classID, "class-id", 0, "curseforge class id (default: 6, mods)")
	f.IntVar(&searchFlags.categoryID, "category-id", 0, "curseforge category id, overrides --category")
	f.IntVar(&searchFlags.modID, "mod-id", 0, "curseforge mod id: fetch that mod directly instead of searching")
	f.StringVar(&searchFlags.slug, "slug", "", "curseforge slug")
	f.IntVar(&searchFlags.gameVersionTypeID, "game-version-type-id", 0, "curseforge game version type: 1=release, 2=beta, 3=alpha")
	f.StringSliceVar(&searchFlags.rawParams, "raw-param", nil, "raw curseforge query parameter key=value, passed through verbatim (repeatable)")

	searchCmd.SetUsageFunc(searchUsageFunc)
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	plat := strings.ToLower(searchFlags.platform)
	if plat == "" {
		return fmt.Errorf("--platform is required (modrinth or curseforge)")
	}
	if err := validateSearchFlags(plat); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client, err := platform.New(plat, cfg)
	if err != nil {
		return err
	}

	query := ""
	if len(args) == 1 {
		query = args[0]
	}
	var openSource *bool
	switch {
	case searchFlags.openSource:
		t := true
		openSource = &t
	case searchFlags.noOpenSource:
		t := false
		openSource = &t
	}

	opts := platform.SearchOptions{
		Query:             query,
		ProjectType:       searchFlags.projectType,
		Environment:       searchFlags.environment,
		Author:            searchFlags.author,
		License:           searchFlags.license,
		Categories:        searchFlags.categories,
		Loaders:           searchFlags.loaders,
		GameVersions:      searchFlags.gameVersions,
		OpenSource:        openSource,
		Sort:              searchFlags.sort,
		SortOrder:         searchFlags.sortOrder,
		Limit:             searchFlags.limit,
		Offset:            searchFlags.offset,
		Facets:            searchFlags.facets,
		FacetsJSON:        searchFlags.facetsJSON,
		ClassID:           searchFlags.classID,
		CategoryID:        searchFlags.categoryID,
		ModID:             searchFlags.modID,
		Slug:              searchFlags.slug,
		GameVersionTypeID: searchFlags.gameVersionTypeID,
		RawParams:         searchFlags.rawParams,
	}
	projects, err := client.Search(cmd.Context(), opts)
	if err != nil {
		return err
	}
	output.Projects(projects, jsonOutput)
	return nil
}

// validateSearchFlags rejects flags that a platform does not support,
// and cross-flag conflicts.
func validateSearchFlags(plat string) error {
	switch plat {
	case platform.PlatformModrinth:
		if searchFlags.sortOrder != "" {
			return fmt.Errorf("--sort-order is only supported on curseforge")
		}
		if searchFlags.classID != 0 {
			return fmt.Errorf("--class-id is only supported on curseforge")
		}
		if searchFlags.categoryID != 0 {
			return fmt.Errorf("--category-id is only supported on curseforge")
		}
		if searchFlags.modID != 0 {
			return fmt.Errorf("--mod-id is only supported on curseforge")
		}
		if searchFlags.slug != "" {
			return fmt.Errorf("--slug is only supported on curseforge")
		}
		if searchFlags.gameVersionTypeID != 0 {
			return fmt.Errorf("--game-version-type-id is only supported on curseforge")
		}
		if len(searchFlags.rawParams) > 0 {
			return fmt.Errorf("--raw-param is only supported on curseforge")
		}
	case platform.PlatformCurseForge:
		if len(searchFlags.facets) > 0 {
			return fmt.Errorf("--facet is only supported on modrinth")
		}
		if searchFlags.facetsJSON != "" {
			return fmt.Errorf("--facets-json is only supported on modrinth")
		}
		if searchFlags.openSource || searchFlags.noOpenSource {
			return fmt.Errorf("--open-source/--no-open-source are only supported on modrinth")
		}
		if searchFlags.environment != "" {
			return fmt.Errorf("--environment is only supported on modrinth")
		}
		if searchFlags.author != "" {
			return fmt.Errorf("--author is not supported by the curseforge search API")
		}
		if searchFlags.license != "" {
			return fmt.Errorf("--license is only supported on modrinth")
		}
	default:
		return fmt.Errorf("unknown platform %q (supported: modrinth, curseforge)", plat)
	}
	if searchFlags.openSource && searchFlags.noOpenSource {
		return fmt.Errorf("--open-source and --no-open-source are mutually exclusive")
	}
	return nil
}
