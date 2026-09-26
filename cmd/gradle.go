package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"cubehaul/internal/gradle"
	"cubehaul/internal/output"
)

// notice writes one explanatory line to stderr. Results stay on stdout, so
// anything that only describes what the command did belongs here.
func notice(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cubehaul: "+format+"\n", args...)
}

// addGradleFlag registers --gradle with an optional value: a bare --gradle
// starts at the working directory, --gradle=<file|dir> starts there instead.
func addGradleFlag(f *pflag.FlagSet, value *string) {
	f.StringVar(value, "gradle", "", "take --loader/--game-version from gradle.properties (bare: search this directory upwards)")
	f.Lookup("gradle").NoOptDefVal = "."
}

// applyGradleTarget fills the version filters from the project's
// gradle.properties when --gradle was given. An explicitly passed flag always
// wins, and every decision is reported on stderr: silently narrowing a search is
// worse than a loud line, and a version read from a range is a guess the user
// should get to see.
func applyGradleTarget(gradleFlag string, loaders, gameVersions *[]string) error {
	if gradleFlag == "" {
		return nil
	}
	target, err := gradle.Load(gradleFlag)
	if err != nil {
		return err
	}

	var used []string

	switch {
	case len(*gameVersions) > 0:
		if target.MinecraftVersion != "" {
			notice("--game-version given, ignoring %s from %s", target.MinecraftVersion, target.File)
		}
	case target.MinecraftVersion != "":
		*gameVersions = append(*gameVersions, target.MinecraftVersion)
		used = append(used, fmt.Sprintf("Minecraft %s (%s)", target.MinecraftVersion, target.MinecraftVersionSource))
	default:
		notice("%s: no Minecraft version found", target.File)
	}

	switch {
	case len(*loaders) > 0:
		if len(target.Loaders) > 0 {
			notice("--loader given, ignoring %s from %s", strings.Join(target.Loaders, ", "), target.File)
		}
	case len(target.Loaders) == 1:
		*loaders = append(*loaders, target.Loaders[0])
		used = append(used, "loader "+target.Loaders[0])
	case len(target.Loaders) > 1:
		notice("%s lists several loaders (%s); pass --loader to pick one", target.File, strings.Join(target.Loaders, ", "))
	default:
		notice("%s: no loader found", target.File)
	}

	if len(used) > 0 {
		notice("using %s from %s", strings.Join(used, ", "), target.File)
	}
	return nil
}

// newTargetCmd builds "target [path]": report what a project's gradle.properties
// says its Minecraft version and loader are.
func newTargetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target [path]",
		Short: "Show the Minecraft version and loader a project's gradle.properties targets",
		Long: `Read a project's gradle.properties and report the Minecraft version and mod
loader it targets. This is the same detection --gradle fills --game-version and
--loader with, so it is also how you check why a filter came out the way it did.

<path> defaults to the working directory, and the search walks upwards until it
finds a gradle.properties. Property names are not a fixed list: minecraft_version,
mcVersion, minecraftVersion and prefixed keys such as
neoforge_121_minecraft_version are all understood. When a project only declares a
version range, the lower bound is used and VERSION SOURCE names the property it
came from, so a guess stays visible.`,
		Example: `  cubehaul target
  cubehaul target ../MyMod
  cubehaul target --json=minecraft_version,loaders`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := "."
			if len(args) == 1 {
				start = args[0]
			}
			detected, err := gradle.Load(start)
			if err != nil {
				return err
			}
			if detected.MinecraftVersion == "" && len(detected.Loaders) == 0 {
				return fmt.Errorf("%s: no Minecraft version or mod loader found", detected.File)
			}
			return output.TargetResult(toOutputTarget(detected), outFormat())
		},
	}
	return cmd
}

// toOutputTarget projects a detected target onto the renderer's type.
func toOutputTarget(t *gradle.Target) output.Target {
	return output.Target{
		File:                   t.File,
		MinecraftVersion:       t.MinecraftVersion,
		MinecraftVersionSource: t.MinecraftVersionSource,
		Loaders:                t.Loaders,
	}
}
