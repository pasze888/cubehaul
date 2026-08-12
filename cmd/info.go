package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
	"cubehaul/internal/platform"
)

var infoCmd = &cobra.Command{
	Use:   "info <platform> <id|slug>",
	Short: "Show details of a single project",
	Long: `Show details of a single project. <platform> is modrinth or curseforge;
<id> is a slug or numeric ID on Modrinth, a numeric mod ID on CurseForge
(the ID or slug comes from the search results).`,
	Example: `  cubehaul info modrinth sodium
  cubehaul info curseforge 394468`,
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
		p, err := client.GetProject(cmd.Context(), args[1])
		if err != nil {
			return err
		}
		output.Project(p, jsonOutput)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
