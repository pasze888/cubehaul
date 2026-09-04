package cmd

import (
	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

// newCurseforgeCmd builds the `cubehaul curseforge` platform command, hosting
// the full verb set (search/info/versions/download/categories) scoped to
// CurseForge.
func newCurseforgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "curseforge",
		Aliases: []string{"cf"},
		Short:   "Work with CurseForge (api.curseforge.com)",
		Long: `Operate on CurseForge projects. The official API requires a key: set
CURSEFORGE_API_KEY or add "curseforge_api_key" to ~/.cubehaul/config.json
(get a key at https://console.curseforge.com). Alternatively, point
CURSEFORGE_API_BASE at a keyless read-only cache such as
https://mod.mcimirror.top/curseforge/v1.`,
	}
	cmd.AddCommand(
		newCurseforgeSearchCmd(),
		newInfoCmd(platform.PlatformCurseForge),
		newVersionsCmd(platform.PlatformCurseForge),
		newDownloadCmd(platform.PlatformCurseForge),
		newCategoriesCmd(platform.PlatformCurseForge, true),
	)
	return cmd
}

// curseforgeSearchFlags are the search inputs specific to CurseForge.
type curseforgeSearchFlags struct {
	common            searchCommonFlags
	sortOrder         string
	classID           int
	categoryID        int
	modID             int
	slug              string
	gameVersionTypeID int
	rawParams         []string
}

func newCurseforgeSearchCmd() *cobra.Command {
	s := &curseforgeSearchFlags{}
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search projects on CurseForge",
		Long: `Search projects on CurseForge.

The query is optional: omitting it lists projects filtered by the given flags.

CurseForge has no facet system; --raw-param passes arbitrary query parameters
through verbatim, e.g. --raw-param 'gameVersion=1.20.1'.

With a query and no --sort, results are ranked by relevance (sortField=13).
The sort direction is always sent explicitly when a field has a meaningful one
-- desc for popularity/updated/downloads/relevancy, asc for name/author --
because the API leaves sortOrder undocumented and empirically treats an omitted
one as ascending. --sort-order overrides it. Relevance is undefined for a
term-less filtered search, so those keep the API's own default order.`,
		Example: `  cubehaul curseforge search sodium --loader forge
  cubehaul curseforge search "" --category technology --sort downloads
  cubehaul curseforge search --mod-id 394468`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client := platform.NewCurseForgeClient(cfg)

			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			opts := s.common.toSearchOptions(platform.PlatformCurseForge, query)
			opts.SortOrder = s.sortOrder
			opts.ClassID = s.classID
			opts.CategoryID = s.categoryID
			opts.ModID = s.modID
			opts.Slug = s.slug
			opts.GameVersionTypeID = s.gameVersionTypeID
			opts.RawParams = s.rawParams

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
	f.StringVar(&s.sortOrder, "sort-order", "", "sort direction: asc or desc")
	f.IntVar(&s.classID, "class-id", 0, "curseforge class id (default: 6, mods)")
	f.IntVar(&s.categoryID, "category-id", 0, "curseforge category id, overrides --category")
	f.IntVar(&s.modID, "mod-id", 0, "curseforge mod id: fetch that mod directly instead of searching")
	f.StringVar(&s.slug, "slug", "", "curseforge slug")
	f.IntVar(&s.gameVersionTypeID, "game-version-type-id", 0, "curseforge game version type: 1=release, 2=beta, 3=alpha")
	f.StringSliceVar(&s.rawParams, "raw-param", nil, "raw curseforge query parameter key=value, passed through verbatim (repeatable)")

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return writeGroupedUsage(c, []struct {
			title string
			flags []string
		}{
			{"Common", []string{
				"project-type", "category", "loader", "game-version",
				"sort", "limit", "offset",
			}},
			{"CurseForge only", []string{
				"sort-order", "class-id", "category-id", "mod-id",
				"slug", "game-version-type-id", "raw-param",
			}},
		})
	})
	return cmd
}
