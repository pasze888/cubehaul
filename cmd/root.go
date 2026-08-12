// Package cmd implements the modfetch CLI commands.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// jsonOutput is a persistent flag: print results as JSON instead of tables.
var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "modfetch",
	Short: "Search, inspect and download Minecraft mods from Modrinth and CurseForge",
	Long: `modfetch searches and downloads Minecraft projects from Modrinth and CurseForge.

Typical workflow:
  modfetch search sodium --platform modrinth --loader fabric      # find projects
  modfetch info modrinth sodium                                   # project details
  modfetch versions modrinth sodium --loader fabric               # pick a version
  modfetch download modrinth sodium --latest --output-dir ./mods  # download it

Configuration:
  Modrinth needs no key but requires a User-Agent, which is set automatically.
  CurseForge requires an API key: set CURSEFORGE_API_KEY or add
  "curseforge_api_key" to ~/.modfetch/config.json (get a key at
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
}
