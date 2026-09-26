package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cubehaul/internal/config"
	"cubehaul/internal/output"
)

// newConfigCmd builds the "cubehaul config" command group, which reads and
// writes ~/.cubehaul/config.json so that the file never has to be edited by
// hand. The gh-style split is list/get/set: reading is one verb, writing is
// another, and neither hides which source actually wins at run time.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the configuration file",
		Long: `Read and write the cubehaul configuration file (~/.cubehaul/config.json;
"cubehaul config path" prints the exact location).

An effective value is resolved in this order:
  1. environment variable, e.g. CURSEFORGE_API_KEY
  2. config file entry
  3. built-in default

Commands:
  list             every key with its effective value and its source
  get <key>        print one effective value, verbatim
  set <key> <val>  validate and write one entry
  unset <key>      remove one entry, restoring the default
  path             print the config file location`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(
		newConfigListCmd(),
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigUnsetCmd(),
		newConfigPathCmd(),
	)
	return cmd
}

// newConfigListCmd builds "config list".
func newConfigListCmd() *cobra.Command {
	var showSecrets bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configuration keys and their effective values",
		Long: `List every editable key with the value in effect and where it comes from:

  env      an environment variable overrides everything else
  file     the value comes from the config file
  default  neither the environment nor the file sets it
  unset    no value anywhere

Secret values are masked unless --show-secrets is given.`,
		Example: `  cubehaul config list
  cubehaul config list --show-secrets
  cubehaul config list --json=key,value`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := configRows()
			if err != nil {
				return err
			}
			return output.Config(rows, outFormat(), showSecrets)
		},
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "print secret values in full instead of masking them")
	return cmd
}

// newConfigGetCmd builds "config get".
func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print the effective value of one configuration key",
		Long: `Print the value in effect for <key>, verbatim and without decoration, so it
can be consumed by scripts. A secret is printed in full here: asking for one key
is an explicit request. Use "cubehaul config list" for an overview, where secrets
stay masked.`,
		Example: "  cubehaul config get curseforge_api_base",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, v, err := resolveConfigKey(args[0])
			if err != nil {
				return err
			}
			if f := outFormat(); f.JSON.On {
				return output.ConfigOne(toConfigRow(v), f)
			}
			fmt.Println(v.Value)
			return nil
		},
	}
	return cmd
}

// newConfigSetCmd builds "config set".
func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one configuration key in the config file",
		Long: `Validate <value> and write <key>=<value> into the config file, leaving every
other entry untouched -- including keys written by hand or by a newer version.
The file is replaced atomically, so a failed write cannot leave a half-written
config behind, and on systems with POSIX permissions only you can read it.

The environment still has the last word at run time: run "cubehaul config list"
afterwards to see which source actually wins.`,
		Example: `  cubehaul config set curseforge_api_key "$CF_API_KEY"
  cubehaul config set user_agent "myname/1.0 (me@example.com)"
  cubehaul config set modrinth_api_base https://api.modrinth.com/v2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := lookupConfigKey(args[0])
			if err != nil {
				return err
			}
			value := args[1]
			if err := validateConfigValue(key, value); err != nil {
				return err
			}
			return writeConfigKey(key.Name, value)
		},
	}
	return cmd
}

// newConfigUnsetCmd builds "config unset".
func newConfigUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove one configuration key",
		Long: `Remove <key> from the config file, so that the built-in default (or nothing at
all) applies again. Unsetting a key that is not set is not an error, and does not
rewrite the file.`,
		Example: "  cubehaul config unset curseforge_api_base",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := lookupConfigKey(args[0])
			if err != nil {
				return err
			}
			path, err := config.ConfigPath()
			if err != nil {
				return err
			}
			values, err := config.LoadFile(path)
			if err != nil {
				return err
			}
			if _, ok := values[key.Name]; !ok {
				return nil
			}
			delete(values, key.Name)
			return config.SaveFile(path, values)
		},
	}
	return cmd
}

// newConfigPathCmd builds "config path".
func newConfigPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print the location of the configuration file",
		Long: `Print the path of the configuration file, whether or not it exists yet. The
file is created by the first "cubehaul config set".`,
		Example: `  cubehaul config path
  cubehaul config path --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ConfigPath()
			if err != nil {
				return err
			}
			if f := outFormat(); f.JSON.On {
				return output.JSON(struct {
					Path string `json:"path"`
				}{path})
			}
			fmt.Println(path)
			return nil
		},
	}
	return cmd
}

// configRows resolves every editable key against the environment and the config
// file, in table order.
func configRows() ([]output.ConfigRow, error) {
	path, err := config.ConfigPath()
	if err != nil {
		return nil, err
	}
	values, err := config.LoadFile(path)
	if err != nil {
		return nil, err
	}
	rows := make([]output.ConfigRow, 0, len(config.Keys))
	for _, key := range config.Keys {
		v, err := key.Resolve(values)
		if err != nil {
			return nil, err
		}
		rows = append(rows, toConfigRow(v))
	}
	return rows, nil
}

// resolveConfigKey looks up a key by name and resolves its effective value.
func resolveConfigKey(name string) (config.Key, config.Value, error) {
	key, err := lookupConfigKey(name)
	if err != nil {
		return config.Key{}, config.Value{}, err
	}
	path, err := config.ConfigPath()
	if err != nil {
		return config.Key{}, config.Value{}, err
	}
	values, err := config.LoadFile(path)
	if err != nil {
		return config.Key{}, config.Value{}, err
	}
	v, err := key.Resolve(values)
	if err != nil {
		return config.Key{}, config.Value{}, err
	}
	return key, v, nil
}

// lookupConfigKey resolves a user-supplied key name, listing the valid keys the
// way gh lists the JSON fields a command accepts.
func lookupConfigKey(name string) (config.Key, error) {
	key, ok := config.LookupKey(name)
	if ok {
		return key, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "unknown config key %q\nvalid keys:\n", name)
	for _, valid := range config.Keys {
		fmt.Fprintf(&b, "  %-22s %s\n", valid.Name, valid.Help)
	}
	return config.Key{}, errors.New(strings.TrimRight(b.String(), "\n"))
}

// validateConfigValue rejects a value the tool could not use, pointing at the
// default when there is one to fall back to.
func validateConfigValue(key config.Key, value string) error {
	if key.Validate == nil {
		return nil
	}
	err := key.Validate(value)
	if err == nil {
		return nil
	}
	hint := ""
	if def := key.Default(); def != "" {
		hint = fmt.Sprintf(" (run \"cubehaul config unset %s\" to fall back to the default)", key.Name)
	}
	return fmt.Errorf("invalid value for %s: %w%s", key.Name, err, hint)
}

// writeConfigKey stores one string value, keeping the rest of the file as is.
func writeConfigKey(name, value string) error {
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}
	values, err := config.LoadFile(path)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	values[name] = encoded
	return config.SaveFile(path, values)
}

// toConfigRow projects a resolved value onto the renderer's row type.
func toConfigRow(v config.Value) output.ConfigRow {
	return output.ConfigRow{
		Key:    v.Key,
		Value:  v.Value,
		Source: string(v.Source),
		Secret: v.Secret,
	}
}
