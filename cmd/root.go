// Package cmd implements the cubehaul CLI commands.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"cubehaul/internal/debug"
	"cubehaul/internal/output"
	"cubehaul/internal/version"
)

// Persistent output flags, shared by every command.
var (
	// jsonOutput is "" when JSON was not requested, "all" for a bare --json
	// (cobra's NoOptDefVal) and otherwise a comma-separated field list.
	jsonOutput string
	// jqOutput filters the JSON document with a jq expression.
	jqOutput string
	// debugOutput mirrors --debug; CUBEHAUL_DEBUG does the same thing.
	debugOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "cubehaul",
	Short: "Search, inspect and download Minecraft mods from Modrinth and CurseForge",
	Long: `cubehaul searches and downloads Minecraft projects from Modrinth and CurseForge.

Platform commands live under a per-platform sub-command, each exposing only the
flags its platform supports; "config" and "target" are not platform-specific:

  cubehaul modrinth search sodium --loader fabric --limit 5
  cubehaul modrinth view sodium
  cubehaul modrinth versions sodium --loader fabric
  cubehaul modrinth download sodium --latest --output-dir ./mods
  cubehaul modrinth categories
  cubehaul modrinth api /project/sodium --jq .downloads

  cubehaul curseforge search sodium --loader neoforge
  cubehaul curseforge view 394468
  cubehaul curseforge versions 394468 --loader neoforge
  cubehaul curseforge download 394468 --version-id 8793728
  cubehaul curseforge categories --class-id 6

Shorthands (identical to the long names):
  cubehaul mr ...  ==  cubehaul modrinth ...
  cubehaul cf ...  ==  cubehaul curseforge ...

Mod projects:
  Inside a mod project, --gradle reads gradle.properties and fills
  --game-version and --loader from it. "cubehaul target" reports what that
  detection finds without making a request:

    cubehaul target
    cubehaul modrinth download sodium --latest --gradle

Output:
  Results print as tables or plain lines for humans; --json prints JSON instead
  and --json=field,field keeps only the named fields (an unknown field name
  lists the ones that exist). --jq <expr> filters that JSON with jq syntax,
  evaluated in-process. --debug prints request, retry and proxy diagnostics to
  stderr.

Configuration:
  Modrinth needs no key but requires a User-Agent, which is set automatically.
  CurseForge's official API requires a key: set CURSEFORGE_API_KEY or add
  "curseforge_api_key" to ~/.cubehaul/config.json (get a key at
  https://console.curseforge.com).
  The config file may also contain a "user_agent" field with your contact info.

  Environment variables win over the file, which wins over the built-in
  defaults. "cubehaul config list" shows every key with the source in effect;
  "cubehaul config set <key> <value>" writes one entry without hand-editing JSON.`,
	Version: version.Value(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug.Set(debugOutput)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	f := rootCmd.PersistentFlags()
	f.StringVar(&jsonOutput, "json", "", "print output as JSON; bare --json keeps every field, --json=field,field keeps only those")
	f.Lookup("json").NoOptDefVal = "all"
	f.StringVar(&jqOutput, "jq", "", "filter the JSON output with a jq expression (implies --json)")
	f.BoolVar(&debugOutput, "debug", false, "print request, retry and proxy diagnostics to stderr")

	rootCmd.AddCommand(newModrinthCmd())
	rootCmd.AddCommand(newCurseforgeCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newTargetCmd())
}

// outFormat builds the shared output format from the global --json/--jq flags.
func outFormat() output.Format {
	return output.NewFormat(jsonOutput, jqOutput)
}
