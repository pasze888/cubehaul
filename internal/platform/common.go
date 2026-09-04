// Package platform provides unified access to Modrinth and CurseForge.
package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cubehaul/internal/config"
)

// Platform names.
const (
	PlatformModrinth   = "modrinth"
	PlatformCurseForge = "curseforge"
)

// SearchOptions carries all search inputs, both platform-agnostic and
// platform-specific. Fields not supported by a platform are rejected with a
// clear error before any request is made.
type SearchOptions struct {
	Query string

	// PlatformName is the target platform ("modrinth" or "curseforge"),
	// set by the calling client so the Search implementation can reject
	// fields it does not implement instead of silently ignoring them.
	PlatformName string

	// Common convenience filters.
	ProjectType  string
	Environment  string
	Author       string
	License      string
	Categories   []string
	Loaders      []string
	GameVersions []string
	OpenSource   *bool
	Sort         string
	Limit        int
	Offset       int

	// Modrinth-only: raw facet passthrough.
	Facets     []string
	FacetsJSON string

	// CurseForge-only.
	SortOrder         string // asc | desc
	ClassID           int
	CategoryID        int
	ModID             int
	Slug              string
	GameVersionTypeID int
	RawParams         []string // --raw-param 'key=value'
}

// Project is a unified view of a mod project.
type Project struct {
	Platform    string   `json:"platform"`
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Downloads   int64    `json:"downloads"`
	Follows     int64    `json:"follows"`
	Categories  []string `json:"categories"`
	License     string   `json:"license"`
	URL         string   `json:"url"`
	UpdatedAt   string   `json:"updated_at"`
}

// File is a downloadable artifact of a version.
type File struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Primary  bool   `json:"primary"`
}

// Version is a unified view of a project release.
type Version struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	DatePublished string   `json:"date_published"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	Files         []File   `json:"files"`
	Changelog     string   `json:"changelog"`
}

// Category is a taxonomy node.
type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ClassID  int    `json:"class_id,omitempty"`
	ParentID int    `json:"parent_id,omitempty"`
	IsClass  bool   `json:"is_class,omitempty"`
}

// Platform is the unified interface implemented by each provider client.
type Platform interface {
	Name() string
	Search(ctx context.Context, o SearchOptions) ([]Project, error)
	GetProject(ctx context.Context, id string) (*Project, error)
	ListVersions(ctx context.Context, projectID string, loaders, gameVersions []string) ([]Version, error)
	Categories(ctx context.Context, classID int) ([]Category, error)
}

// New creates a client for the named platform.
func New(name string, cfg *config.Config) (Platform, error) {
	switch name {
	case PlatformModrinth:
		return NewModrinthClient(cfg), nil
	case PlatformCurseForge:
		return NewCurseForgeClient(cfg), nil
	default:
		return nil, fmt.Errorf("unknown platform %q (supported: %s, %s)", name, PlatformModrinth, PlatformCurseForge)
	}
}

// sortVersionsByDate orders versions newest-first by DatePublished.
func sortVersionsByDate(vs []Version) {
	sort.SliceStable(vs, func(i, j int) bool {
		return vs[i].DatePublished > vs[j].DatePublished
	})
}

// anyFold reports whether haystack contains needle, case-insensitively.
func anyFold(haystack, needles []string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if strings.EqualFold(h, n) {
				return true
			}
		}
	}
	return false
}

// anyExact reports whether haystack contains any of needles, exactly.
func anyExact(haystack, needles []string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if h == n {
				return true
			}
		}
	}
	return false
}

// Per-request search limits enforced by each platform's API. Modrinth
// silently clamps above 100; CurseForge /mods/search rejects pageSize > 50
// with an HTTP 500, so clamping there also keeps the retry logic from
// re-issuing a request that can never succeed.
const (
	ModrinthMaxSearchLimit   = 100
	CurseForgeMaxSearchLimit = 50
)

// clampSearchLimit caps a requested --limit at the platform's per-request
// max, returning the effective value and a stderr notice when capped. Search
// stays single-page by design; callers paginate with --offset.
func clampSearchLimit(platformName string, limit, max int) (effective int, notice string) {
	if limit <= max {
		return limit, ""
	}
	return max, fmt.Sprintf("cubehaul: %s: --limit %d exceeds the per-request max of %d; showing %d (page with --offset)\n",
		platformName, limit, max, max)
}
