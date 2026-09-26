package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"cubehaul/internal/browser"
	"cubehaul/internal/platform"
)

// openInBrowser opens u and reports what it did on stderr, keeping stdout for
// results so that a --web run stays pipeable.
func openInBrowser(u string) error {
	if err := browser.Open(u); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Opening %s in your browser.\n", u)
	return nil
}

// openProjectInBrowser opens the page of a project the API just returned,
// complaining when that response carried no URL.
func openProjectInBrowser(u, id string) error {
	if u == "" {
		return fmt.Errorf("the API returned no web URL for %s", id)
	}
	return openInBrowser(u)
}

// searchWebURL builds the human-facing search page for a query. Only the query
// and the project type carry over: the platform web pages encode their filters
// differently from the API, so mapping the rest would be guesswork.
func searchWebURL(plat, query, projectType string) string {
	switch plat {
	case platform.PlatformModrinth:
		site := "https://modrinth.com/" + modrinthSection(projectType)
		if query == "" {
			return site
		}
		return site + "?q=" + url.QueryEscape(query)
	case platform.PlatformCurseForge:
		site := "https://www.curseforge.com/minecraft/search"
		if query == "" {
			return site
		}
		return site + "?search=" + url.QueryEscape(query)
	default:
		return ""
	}
}

// modrinthSection maps a project type onto the matching Modrinth browse path.
func modrinthSection(projectType string) string {
	switch strings.ToLower(projectType) {
	case "modpack", "modpacks":
		return "modpacks"
	case "resourcepack", "resourcepacks":
		return "resourcepacks"
	case "shader", "shaders":
		return "shaders"
	case "plugin", "plugins":
		return "plugins"
	case "datapack", "datapacks":
		return "datapacks"
	default:
		return "mods"
	}
}
