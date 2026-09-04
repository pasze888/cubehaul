package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

// newModrinthCmd builds the `cubehaul modrinth` platform command, hosting the
// full verb set (search/info/versions/download/categories) scoped to Modrinth.
func newModrinthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "modrinth",
		Aliases: []string{"mr"},
		Short:   "Work with Modrinth (api.modrinth.com)",
		Long: `Operate on Modrinth projects. No authentication is required; a User-Agent
is set automatically. Set MODRINTH_API_BASE (or "modrinth_api_base" in
~/.cubehaul/config.json) to point at a mirror/cache such as
https://mod.mcimirror.top/modrinth/v2.`,
	}
	cmd.AddCommand(
		newModrinthSearchCmd(),
		newInfoCmd(platform.PlatformModrinth),
		newVersionsCmd(platform.PlatformModrinth),
		newDownloadCmd(platform.PlatformModrinth),
		newCategoriesCmd(platform.PlatformModrinth, false),
	)
	return cmd
}

// modrinthSearchFlags are the search inputs specific to Modrinth.
type modrinthSearchFlags struct {
	common      searchCommonFlags
	openSource  bool
	noOpen      bool
	environment string
	author      string
	license     string
	facets      []string
	facetsJSON  string
}

func newModrinthSearchCmd() *cobra.Command {
	s := &modrinthSearchFlags{}
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search projects on Modrinth",
		Long: `Search projects on Modrinth.

The query is optional: omitting it lists projects filtered by the given flags.

Facets are Modrinth's precise filter system. --category, --loader and
--game-version are convenience flags that expand to facets; --facet passes
raw facet strings ("downloads<=100", "versions!=1.20.1") and --facets-json
passes a raw JSON array of arrays, e.g. [["categories:forge"],["versions:1.17.1"]].`,
		Example: `  cubehaul modrinth search sodium --loader fabric --limit 5
  cubehaul modrinth search "" --category adventure --sort downloads
  cubehaul modrinth search "" --facet 'downloads>=100000000' --sort downloads`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.openSource && s.noOpen {
				return fmt.Errorf("--open-source and --no-open-source are mutually exclusive")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client := platform.NewModrinthClient(cfg)

			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			opts := s.common.toSearchOptions(platform.PlatformModrinth, query)
			var openSource *bool
			switch {
			case s.openSource:
				t := true
				openSource = &t
			case s.noOpen:
				t := false
				openSource = &t
			}
			opts.OpenSource = openSource
			opts.Environment = s.environment
			opts.Author = s.author
			opts.License = s.license
			opts.Facets = s.facets
			opts.FacetsJSON = s.facetsJSON

			projects, err := client.Search(cmd.Context(), opts)
			if err != nil {
				return err
			}
			output.Projects(projects, jsonOutput)
			return nil
		},
	}

	f := cmd.Flags()
	addSearchCommonFlags(f, &s.common)
	f.BoolVar(&s.openSource, "open-source", false, "only open-source projects")
	f.BoolVar(&s.noOpen, "no-open-source", false, "only closed-source projects")
	f.StringVar(&s.environment, "environment", "", "supported environment: client, server, client_and_server")
	f.StringVar(&s.author, "author", "", "filter by author username")
	f.StringVar(&s.license, "license", "", "filter by SPDX license id, e.g. mit")
	f.StringSliceVar(&s.facets, "facet", nil, "raw facet, e.g. 'downloads<=100' or 'versions!=1.20.1' (repeatable)")
	f.StringVar(&s.facetsJSON, "facets-json", "", "raw facets as JSON array of arrays, e.g. '[[\"categories:forge\"],[\"versions:1.17.1\"]]'")

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return writeGroupedUsage(c, []struct {
			title string
			flags []string
		}{
			{"Common", []string{
				"project-type", "category", "loader", "game-version",
				"sort", "limit", "offset",
			}},
			{"Modrinth facets", []string{
				"open-source", "no-open-source", "environment",
				"author", "license", "facet", "facets-json",
			}},
		})
	})
	return cmd
}
