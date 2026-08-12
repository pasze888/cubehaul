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

var downloadFlags struct {
	versionID    string
	latest       bool
	loaders      []string
	gameVersions []string
	outputDir    string
}

var downloadCmd = &cobra.Command{
	Use:   "download <platform> <id>",
	Short: "Download a version of a project",
	Long: `Download a version of a project. <platform> is modrinth or curseforge;
<id> comes from the search results. Choose the version with one of:
  --version-id <id>                exact version id (see "cubehaul versions")
  --latest                         most recent version
  --loader / --game-version        first version matching the filters

If several files exist for the version, the primary file is downloaded.`,
	Example: `  cubehaul download modrinth sodium --latest
  cubehaul download modrinth sodium --loader fabric --game-version 1.20.1
  cubehaul download curseforge 394468 --version-id 5730579 --output-dir ./mods`,
	Args: cobra.ExactArgs(2),
	RunE: runDownload,
}

func init() {
	f := downloadCmd.Flags()
	f.StringVar(&downloadFlags.versionID, "version-id", "", "exact version id to download")
	f.BoolVar(&downloadFlags.latest, "latest", false, "download the latest version")
	f.StringSliceVar(&downloadFlags.loaders, "loader", nil, "require this mod loader (repeatable)")
	f.StringSliceVar(&downloadFlags.gameVersions, "game-version", nil, "require this Minecraft version (repeatable)")
	f.StringVar(&downloadFlags.outputDir, "output-dir", "./mods", "directory to save the file into")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	if downloadFlags.versionID != "" && downloadFlags.latest {
		return fmt.Errorf("--version-id and --latest are mutually exclusive")
	}
	if downloadFlags.versionID == "" && !downloadFlags.latest &&
		len(downloadFlags.loaders) == 0 && len(downloadFlags.gameVersions) == 0 {
		return fmt.Errorf("choose a version: --version-id <id>, --latest, or --loader/--game-version")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client, err := platform.New(strings.ToLower(args[0]), cfg)
	if err != nil {
		return err
	}

	// --version-id targets an exact release: do not narrow the list first.
	var filters []string
	if downloadFlags.versionID == "" {
		filters = append(filters, downloadFlags.loaders...)
	}
	versions, err := client.ListVersions(cmd.Context(), args[1], filters, downloadFlags.gameVersions)
	if err != nil {
		return err
	}

	var target *platform.Version
	switch {
	case downloadFlags.versionID != "":
		for i := range versions {
			if versions[i].ID == downloadFlags.versionID {
				target = &versions[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("version %q not found for project %s (see `cubehaul versions %s %s`)",
				downloadFlags.versionID, args[1], args[0], args[1])
		}
	case downloadFlags.latest:
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for project %s", args[1])
		}
		target = &versions[0]
	default:
		for i := range versions {
			if len(downloadFlags.loaders) > 0 && !matchesAny(versions[i].Loaders, downloadFlags.loaders) {
				continue
			}
			if len(downloadFlags.gameVersions) > 0 && !matchesAny(versions[i].GameVersions, downloadFlags.gameVersions) {
				continue
			}
			target = &versions[i]
			break
		}
		if target == nil {
			return fmt.Errorf("no version matches loaders %v and game versions %v", downloadFlags.loaders, downloadFlags.gameVersions)
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

	res, err := download.Run(cmd.Context(), download.Options{
		URL:      file.URL,
		Filename: file.Filename,
		Dir:      downloadFlags.outputDir,
		Size:     file.Size,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded %s (%s)\n", res.Path, output.HumanSize(res.Size))
	fmt.Printf("  project %s | version %s | loaders %s | game versions %s\n",
		args[1], target.VersionNumber,
		strings.Join(target.Loaders, ", "), strings.Join(target.GameVersions, ", "))
	return nil
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
