package cmd

import (
	"testing"

	"cubehaul/internal/platform"
)

func TestSearchWebURL(t *testing.T) {
	cases := []struct {
		plat        string
		query       string
		projectType string
		want        string
	}{
		{platform.PlatformModrinth, "sodium", "", "https://modrinth.com/mods?q=sodium"},
		{platform.PlatformModrinth, "sodium", "modpack", "https://modrinth.com/modpacks?q=sodium"},
		{platform.PlatformModrinth, "sodium", "resourcepack", "https://modrinth.com/resourcepacks?q=sodium"},
		{platform.PlatformModrinth, "", "", "https://modrinth.com/mods"},
		{platform.PlatformModrinth, "a b", "", "https://modrinth.com/mods?q=a+b"},
		{platform.PlatformCurseForge, "sodium", "", "https://www.curseforge.com/minecraft/search?search=sodium"},
		{platform.PlatformCurseForge, "", "modpack", "https://www.curseforge.com/minecraft/search"},
		{"unknown", "sodium", "", ""},
	}
	for _, c := range cases {
		if got := searchWebURL(c.plat, c.query, c.projectType); got != c.want {
			t.Errorf("searchWebURL(%s, %q, %q) = %q, want %q", c.plat, c.query, c.projectType, got, c.want)
		}
	}
}
