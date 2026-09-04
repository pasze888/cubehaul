package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/download"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

// newInfoCmd builds `info <id|slug>` for the given platform. The platform is
// fixed by the enclosing platform sub-command, so only the project id/slug is
// positional.
func newInfoCmd(plat string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <id|slug>",
		Short: "Show details of a single project",
		Long: `Show details of a single project. <id> is a slug or numeric ID on Modrinth,
a numeric mod ID on CurseForge (comes from the search results).`,
		Example: fmt.Sprintf("  cubehaul %s info sodium", plat),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newPlatformClient(plat)
			if err != nil {
				return err
			}
			p, err := client.GetProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			output.Project(p, jsonOutput)
			return nil
		},
	}
	return cmd
}

// newVersionsCmd builds `versions <id>` for the given platform.
func newVersionsCmd(plat string) *cobra.Command {
	var flags struct {
		loaders      []string
		gameVersions []string
		limit        int
	}
	cmd := &cobra.Command{
		Use:   "versions <id>",
		Short: "List versions of a project",
		Long: `List versions (releases/files) of a project. The ID column feeds directly
into "download --version-id".`,
		Example: fmt.Sprintf("  cubehaul %s versions sodium --loader fabric", plat),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newPlatformClient(plat)
			if err != nil {
				return err
			}
			versions, err := client.ListVersions(cmd.Context(), args[0], flags.loaders, flags.gameVersions)
			if err != nil {
				return err
			}
			if len(versions) > flags.limit {
				versions = versions[:flags.limit]
			}
			output.Versions(versions, jsonOutput)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&flags.loaders, "loader", nil, "only versions for this loader (repeatable)")
	f.StringSliceVar(&flags.gameVersions, "game-version", nil, "only versions for this game version (repeatable)")
	f.IntVar(&flags.limit, "limit", 50, "max versions to show")
	return cmd
}

// newDownloadCmd builds `download <id>` for the given platform.
func newDownloadCmd(plat string) *cobra.Command {
	var flags struct {
		versionID    string
		latest       bool
		loaders      []string
		gameVersions []string
		outputDir    string
	}
	cmd := &cobra.Command{
		Use:   "download <id>",
		Short: "Download a version of a project",
		Long: `Download a version of a project. <id> comes from the search results. Choose
the version with one of:
  --version-id <id>                exact version id (see "versions")
  --latest                         most recent version
  --loader / --game-version        first version matching the filters

If several files exist for the version, the primary file is downloaded.

Files are saved into the system Downloads folder unless --output-dir is given.`,
		Example: fmt.Sprintf("  cubehaul %s download sodium --latest\n  cubehaul %s download sodium --loader fabric --game-version 1.20.1", plat, plat),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.versionID != "" && flags.latest {
				return fmt.Errorf("--version-id and --latest are mutually exclusive")
			}
			if flags.versionID == "" && !flags.latest &&
				len(flags.loaders) == 0 && len(flags.gameVersions) == 0 {
				return fmt.Errorf("choose a version: --version-id <id>, --latest, or --loader/--game-version")
			}

			client, err := newPlatformClient(plat)
			if err != nil {
				return err
			}

			// --version-id targets an exact release: do not narrow the list first.
			var filters []string
			if flags.versionID == "" {
				filters = append(filters, flags.loaders...)
			}
			versions, err := client.ListVersions(cmd.Context(), args[0], filters, flags.gameVersions)
			if err != nil {
				return err
			}

			var target *platform.Version
			switch {
			case flags.versionID != "":
				for i := range versions {
					if versions[i].ID == flags.versionID {
						target = &versions[i]
						break
					}
				}
				if target == nil {
					return fmt.Errorf("version %q not found for project %s (see `cubehaul %s versions %s`)",
						flags.versionID, args[0], plat, args[0])
				}
			case flags.latest:
				if len(versions) == 0 {
					return fmt.Errorf("no versions found for project %s", args[0])
				}
				target = &versions[0]
			default:
				for i := range versions {
					if len(flags.loaders) > 0 && !matchesAny(versions[i].Loaders, flags.loaders) {
						continue
					}
					if len(flags.gameVersions) > 0 && !matchesAny(versions[i].GameVersions, flags.gameVersions) {
						continue
					}
					target = &versions[i]
					break
				}
				if target == nil {
					return fmt.Errorf("no version matches loaders %v and game versions %v", flags.loaders, flags.gameVersions)
				}
			}

			file := target.Files[0]
			for _, f := range target.Files {
				if f.Primary {
					file = f
					break
				}
			}
			if file.URL == "" {
				return fmt.Errorf("version %s has no downloadable file", target.ID)
			}

			dir := flags.outputDir
			if dir == "" {
				dir, err = download.DefaultDir()
				if err != nil {
					return fmt.Errorf("cannot locate the system Downloads folder, pass --output-dir: %w", err)
				}
			}

			res, err := download.Run(cmd.Context(), download.Options{
				URL:      file.URL,
				Filename: file.Filename,
				Dir:      dir,
				Size:     file.Size,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Downloaded %s (%s)\n", res.Path, output.HumanSize(res.Size))
			fmt.Printf("  project %s | version %s | loaders %s | game versions %s\n",
				args[0], target.VersionNumber,
				strings.Join(target.Loaders, ", "), strings.Join(target.GameVersions, ", "))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&flags.versionID, "version-id", "", "exact version id to download")
	f.BoolVar(&flags.latest, "latest", false, "download the latest version")
	f.StringSliceVar(&flags.loaders, "loader", nil, "require this mod loader (repeatable)")
	f.StringSliceVar(&flags.gameVersions, "game-version", nil, "require this Minecraft version (repeatable)")
	f.StringVar(&flags.outputDir, "output-dir", "", outputDirUsage())
	return cmd
}

// outputDirUsage builds the --output-dir help text, spelling out the resolved
// system Downloads path so users see where files actually land. When it cannot
// be determined the flag stays documented generically; the real error surfaces
// at run time, not while building the command tree.
func outputDirUsage() string {
	if dir, err := download.DefaultDir(); err == nil {
		return fmt.Sprintf("directory to save the file into (default: %s)", dir)
	}
	return "directory to save the file into (default: system Downloads folder)"
}

// newCategoriesCmd builds `categories` for the given platform. The class-id
// flag is only meaningful for CurseForge, so it is exposed only there.
func newCategoriesCmd(plat string, exposeClassID bool) *cobra.Command {
	var flags struct {
		classID int
	}
	cmd := &cobra.Command{
		Use:     "categories",
		Short:   "List Minecraft categories",
		Long:    `List the Minecraft category tree. Names feed into "search --category".`,
		Example: fmt.Sprintf("  cubehaul %s categories", plat),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newPlatformClient(plat)
			if err != nil {
				return err
			}
			cats, err := client.Categories(cmd.Context(), flags.classID)
			if err != nil {
				return err
			}
			output.Categories(cats, jsonOutput)
			return nil
		},
	}
	if exposeClassID {
		cmd.Flags().IntVar(&flags.classID, "class-id", 0, "only show categories of this class (curseforge; default: all)")
	}
	return cmd
}

// newPlatformClient loads config and constructs the client for plat.
func newPlatformClient(plat string) (platform.Platform, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return platform.New(plat, cfg)
}

// matchesAny reports whether haystack contains any needle, case-insensitively.
func matchesAny(haystack, needles []string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if strings.EqualFold(h, n) {
				return true
			}
		}
	}
	return false
}
