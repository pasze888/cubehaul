package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"modfetch/internal/config"
	"modfetch/internal/output"
	"modfetch/internal/platform"
)

var categoriesFlags struct {
	classID int
}

var categoriesCmd = &cobra.Command{
	Use:   "categories <platform>",
	Short: "List Minecraft categories",
	Long: `List the Minecraft category tree. <platform> is modrinth or curseforge.
CurseForge returns classes and their sub-categories (accessible without an
API key); Modrinth returns its category tags. These names feed into
"modfetch search --category".`,
	Example: `  modfetch categories curseforge --class-id 6
  modfetch categories modrinth`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		client, err := platform.New(strings.ToLower(args[0]), cfg)
		if err != nil {
			return err
		}
		cats, err := client.Categories(cmd.Context(), categoriesFlags.classID)
		if err != nil {
			return err
		}
		output.Categories(cats, jsonOutput)
		return nil
	},
}

func init() {
	f := categoriesCmd.Flags()
	f.IntVar(&categoriesFlags.classID, "class-id", 0, "only show categories of this class (curseforge; default: all)")
	rootCmd.AddCommand(categoriesCmd)
}
