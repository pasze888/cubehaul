package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"cubehaul/internal/platform"
)

// newAPICmd builds "api <path>": a passthrough for API endpoints the typed
// commands do not cover, so the CLI never becomes the limiting factor.
func newAPICmd(plat string) *cobra.Command {
	var fields []string
	example := "  cubehaul modrinth api /project/sodium --jq .downloads"
	if plat == platform.PlatformCurseForge {
		example = "  cubehaul curseforge api /mods/394468 --jq .data.downloadCount"
	}
	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Call a platform API endpoint directly and print the JSON response",
		Long: `Call an arbitrary GET endpoint of the platform API and print the JSON response.
It is the escape hatch for anything the typed commands do not cover: pass query
parameters with --field/-f and pick out what you need with --jq.

The path is relative to the platform's API root, with or without a leading "/"
(cubehaul modrinth api /project/sodium). Authentication, the User-Agent, the
system proxy and retries all behave exactly as for every other request.

Only GET is supported: cubehaul reads, it does not write.`,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := outFormat()
			if f.JSON.On && !f.JSON.All {
				return errors.New("api prints the response as it arrives; use --jq to pick fields instead of --json=...")
			}

			params := url.Values{}
			for _, pair := range fields {
				key, value, ok := strings.Cut(pair, "=")
				if !ok || key == "" {
					return fmt.Errorf("--field %q must be key=value", pair)
				}
				params.Add(key, value)
			}

			client, err := newPlatformClient(plat)
			if err != nil {
				return err
			}
			path := args[0]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			doc, err := client.DoRaw(cmd.Context(), path, params)
			if err != nil {
				return err
			}
			return f.Document(doc)
		},
	}
	cmd.Flags().StringSliceVarP(&fields, "field", "f", nil, "query parameter key=value (repeatable)")
	return cmd
}
