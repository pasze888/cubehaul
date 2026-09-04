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

All verbs live under a per-platform sub-command, each exposing only the flags
its platform supports:

  cubehaul modrinth search sodium --loader fabric --limit 5
  cubehaul modrinth info sodium
  cubehaul modrinth versions sodium --loader fabric
  cubehaul modrinth download sodium --latest --output-dir ./mods
  cubehaul modrinth categories

  cubehaul curseforge search sodium --loader neoforge
  cubehaul curseforge info 394468
  cubehaul curseforge versions 394468 --loader neoforge
  cubehaul curseforge download 394468 --version-id 8793728
  cubehaul curseforge categories --class-id 6

Shorthands (identical to the long names):
  cubehaul mr ...  ==  cubehaul modrinth ...
  cubehaul cf ...  ==  cubehaul curseforge ...

Configuration:
  Modrinth needs no key but requires a User-Agent, which is set automatically.
  CurseForge's official API requires a key: set CURSEFORGE_API_KEY or add
  "curseforge_api_key" to ~/.cubehaul/config.json (get a key at
  https://console.curseforge.com). Without a key you can still use a keyless
  read-only cache/mirror (e.g. MCIM) by pointing CURSEFORGE_API_BASE and
  MODRINTH_API_BASE (or the "curseforge_api_base"/"modrinth_api_base" config
  fields) at its endpoints, e.g. https://mod.mcimirror.top/curseforge/v1.
  The config file may also contain a "user_agent" field with your contact info.`,
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
