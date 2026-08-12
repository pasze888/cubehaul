package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"modfetch/internal/config"
	"modfetch/internal/output"
	"modfetch/internal/platform"
)

var versionsFlags struct {
	loaders      []string
	gameVersions []string
	limit        int
}

var versionsCmd = &cobra.Command{
	Use:   "versions <platform> <id>",
	Short: "List versions of a project",
	Long: `List versions (releases/files) of a project. <platform> is modrinth or
curseforge; <id> is a slug or ID on Modrinth, a numeric mod ID on CurseForge.
The ID column feeds directly into "modfetch download --version-id".`,
	Example: `  modfetch versions modrinth sodium --loader fabric
  modfetch versions curseforge 394468 --game-version 1.20.1`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		client, err := platform.New(strings.ToLower(args[0]), cfg)
		if err != nil {
			return err
		}
		versions, err := client.ListVersions(cmd.Context(), args[1], versionsFlags.loaders, versionsFlags.gameVersions)
		if err != nil {
			return err
		}
		if len(versions) > versionsFlags.limit {
			versions = versions[:versionsFlags.limit]
		}
		output.Versions(versions, jsonOutput)
		return nil
	},
}

func init() {
	f := versionsCmd.Flags()
	f.StringSliceVar(&versionsFlags.loaders, "loader", nil, "only versions for this loader (repeatable)")
	f.StringSliceVar(&versionsFlags.gameVersions, "game-version", nil, "only versions for this game version (repeatable)")
	f.IntVar(&versionsFlags.limit, "limit", 50, "max versions to show (curseforge returns at most 50 per request)")
	rootCmd.AddCommand(versionsCmd)
}
