// Package cmd implements the cubehaul CLI commands.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// jsonOutput is a persistent flag: print results as JSON instead of tables.
var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "cubehaul",
	Short: "Search, inspect and download Minecraft mods from Modrinth and CurseForge",
	Long: `cubehaul searches and downloads Minecraft projects from Modrinth and CurseForge.

Search is available under per-platform sub-commands, each with its own flags:

  cubehaul modrinth search sodium --loader fabric --limit 5
  cubehaul curseforge search sodium --loader forge

The remaining verbs take <platform> as a positional argument:

  cubehaul info modrinth sodium                                   # project details
  cubehaul versions modrinth sodium --loader fabric               # pick a version
  cubehaul download modrinth sodium --latest --output-dir ./mods  # download it
  cubehaul categories modrinth                                    # list categories

Configuration:
  Modrinth needs no key but requires a User-Agent, which is set automatically.
  CurseForge requires an API key: set CURSEFORGE_API_KEY or add
  "curseforge_api_key" to ~/.cubehaul/config.json (get a key at
  https://console.curseforge.com). The config file may also contain a
  "user_agent" field with your contact info.`,
	Version: "0.1.0",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "print output as JSON")
	rootCmd.AddCommand(newModrinthCmd())
	rootCmd.AddCommand(newCurseforgeCmd())
}
