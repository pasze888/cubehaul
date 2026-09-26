package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

// newCurseforgeCmd builds the "cubehaul curseforge" platform command, hosting
// the full verb set (search/view/versions/download/categories/api) scoped to
// CurseForge.
func newCurseforgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "curseforge",
		Aliases: []string{"cf"},
		Short:   "Work with CurseForge (api.curseforge.com)",
		Long: `Operate on CurseForge projects. The official API requires a key: set
CURSEFORGE_API_KEY or add "curseforge_api_key" to ~/.cubehaul/config.json
(get a key at https://console.curseforge.com).`,
	}
	cmd.AddCommand(
		newCurseforgeSearchCmd(),
		newInfoCmd(platform.PlatformCurseForge),
		newVersionsCmd(platform.PlatformCurseForge),
		newDownloadCmd(platform.PlatformCurseForge),
		newCategoriesCmd(platform.PlatformCurseForge, true),
		newAPICmd(platform.PlatformCurseForge),
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
	web               bool
	gradle            string
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
term-less filtered search, so those keep the API's own default order.

With --web no request is made: the CurseForge search page is opened with the
query only, since the site encodes its other filters differently from the API.`,
		Example: `  cubehaul curseforge search sodium --loader forge
  cubehaul curseforge search --category technology --sort downloads
  cubehaul curseforge search --mod-id 394468`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			if s.web {
				u := searchWebURL(platform.PlatformCurseForge, query, s.common.projectType)
				if u == "" {
					return fmt.Errorf("cannot build a CurseForge search URL")
				}
				return openInBrowser(u)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client := platform.NewCurseForgeClient(cfg)

			opts := s.common.toSearchOptions(platform.PlatformCurseForge, query)
			opts.SortOrder = s.sortOrder
			opts.ClassID = s.classID
			opts.CategoryID = s.categoryID
			opts.ModID = s.modID
			opts.Slug = s.slug
			opts.GameVersionTypeID = s.gameVersionTypeID
			opts.RawParams = s.rawParams

			if err := applyGradleTarget(s.gradle, &opts.Loaders, &opts.GameVersions); err != nil {
				return err
			}

			projects, err := client.Search(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Projects(projects, outFormat())
		},
	}

	f := cmd.Flags()
	addSearchCommonFlags(f, &s.common, platform.PlatformCurseForge)
	f.StringVar(&s.sortOrder, "sort-order", "", "sort direction: asc or desc")
	f.IntVar(&s.classID, "class-id", 0, "class id (default: 6, mods)")
	f.IntVar(&s.categoryID, "category-id", 0, "category id, overrides --category")
	f.IntVar(&s.modID, "mod-id", 0, "mod id: fetch that mod directly instead of searching")
	f.StringVar(&s.slug, "slug", "", "slug")
	f.IntVar(&s.gameVersionTypeID, "game-version-type-id", 0, "game version type: 1=release, 2=beta, 3=alpha")
	f.StringSliceVar(&s.rawParams, "raw-param", nil, "query parameter key=value, passed through verbatim (repeatable)")
	f.BoolVarP(&s.web, "web", "w", false, "open the search page in a browser instead of calling the API")
	addGradleFlag(f, &s.gradle)

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return writeGroupedUsage(c, []struct {
			title string
			flags []string
		}{
			{"Common", []string{
				"project-type", "category", "loader", "game-version",
				"sort", "limit", "offset", "gradle",
			}},
			{"CurseForge only", []string{
				"sort-order", "class-id", "category-id", "mod-id",
				"slug", "game-version-type-id", "raw-param",
			}},
			{"Output", []string{"web"}},
		})
	})
	return cmd
}
