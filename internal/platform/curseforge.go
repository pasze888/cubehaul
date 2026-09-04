package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cubehaul/internal/config"
	"cubehaul/internal/netx"
)

// Minecraft game ID on CurseForge.
const curseForgeGameID = 432

// curseForgeFileStatusApproved is the CurseForge fileStatus enum value that
// marks a file as Approved and therefore downloadable. Other values
// (1=Processing, 2=ChangesRequired, 3=UnderReview, 5=Rejected, 7=Deleted,
// 8=Archived, ...) are files that are not yet or no longer available.
const curseForgeFileStatusApproved = 4

// curseForgeClassIDs maps --project-type values to CurseForge class IDs.
var curseForgeClassIDs = map[string]int{
	"mod":           6,
	"modpack":       4471,
	"resourcepack":  12,
	"shader":        6552,
	"datapack":      6945,
	"plugin":        5,
	"customization": 4546,
	"addons":        4559,
	"world":         17,
	"worlds":        17,
}

// curseForgeLoaderTypes maps --loader values to modLoaderType IDs. The enum is
// fixed by CurseForge and stops at 6: higher ids are rejected outright
// ("The value 'N' is invalid"), and there is no Rift entry despite the loader
// name occasionally showing up in a file's gameVersions list.
var curseForgeLoaderTypes = map[string]int{
	"forge":      1,
	"cauldron":   2,
	"liteloader": 3,
	"fabric":     4,
	"quilt":      5,
	"neoforge":   6,
}

// CurseForge sortField ids (ModsSearchSortField).
const (
	curseForgeSortFeatured    = 1
	curseForgeSortPopularity  = 2
	curseForgeSortLastUpdated = 3
	curseForgeSortName        = 4
	curseForgeSortAuthor      = 5
	curseForgeSortDownloads   = 6
	curseForgeSortCategory    = 7
	curseForgeSortGameVersion = 8
	// curseForgeSortRelevancy ranks by how well a project matches the search
	// term. It is the only query-aware sort field; every other value orders by a
	// fixed project attribute regardless of what was searched for.
	curseForgeSortRelevancy = 13
)

// curseForgeSortFields maps --sort values to sortField IDs.
var curseForgeSortFields = map[string]int{
	"featured":     curseForgeSortFeatured,
	"popularity":   curseForgeSortPopularity,
	"updated":      curseForgeSortLastUpdated,
	"name":         curseForgeSortName,
	"author":       curseForgeSortAuthor,
	"downloads":    curseForgeSortDownloads,
	"category":     curseForgeSortCategory,
	"game-version": curseForgeSortGameVersion,
	"relevancy":    curseForgeSortRelevancy,
}

// curseForgeSortDirections is the direction that makes each field mean what its
// name promises. The spec declares no default for sortOrder and an omitted one
// behaves like asc, so "bigger is better" fields come back inverted unless the
// direction is sent explicitly: --sort downloads would otherwise bury the most
// downloaded mod at the end of the result set.
//
// Featured, category and game version are deliberately absent: they are not
// monotonic quality signals, so the server's own ordering is kept.
var curseForgeSortDirections = map[int]string{
	curseForgeSortPopularity:  "desc",
	curseForgeSortLastUpdated: "desc",
	curseForgeSortDownloads:   "desc",
	curseForgeSortRelevancy:   "desc",
	curseForgeSortName:        "asc",
	curseForgeSortAuthor:      "asc",
}

// cfSortQuery resolves the sortField/sortOrder pair to send for a search.
//
// A free-text query defaults to Relevancy: it is the only field that ranks by
// match quality, so without it "search sodium" returns the server's default
// order and buries the actual Sodium. Relevancy means nothing without a term,
// so filtered listings (empty query) still send no sortField at all.
//
// The direction comes from --sort-order when given, otherwise from
// curseForgeSortDirections, so an implied or explicit sort field is never left
// to the undocumented server default.
func cfSortQuery(o SearchOptions) (int, string, error) {
	order := strings.ToLower(o.SortOrder)
	if order != "" && order != "asc" && order != "desc" {
		return 0, "", fmt.Errorf("invalid --sort-order %q (use asc or desc)", o.SortOrder)
	}

	field := 0
	switch {
	case o.Sort != "":
		f, ok := curseForgeSortFields[strings.ToLower(o.Sort)]
		if !ok {
			return 0, "", fmt.Errorf("unsupported sort %q for curseforge (supported: featured, popularity, updated, name, author, downloads, category, game-version, relevancy)", o.Sort)
		}
		field = f
	case o.Query != "":
		field = curseForgeSortRelevancy
	}
	if field == 0 {
		// Nothing to order by; a lone sortOrder would be meaningless.
		return 0, "", nil
	}
	if order == "" {
		// Missing key yields "": the field has no implied direction.
		order = curseForgeSortDirections[field]
	}
	return field, order, nil
}

// cfLoaderNames are loader names that appear in file gameVersions lists.
var cfLoaderNames = []string{"forge", "fabric", "quilt", "neoforge", "rift", "liteloader"}

// CurseForgeClient talks to api.curseforge.com/v1.
type CurseForgeClient struct {
	base      string
	http      *http.Client
	apiKey    string
	userAgent string
}

// NewCurseForgeClient creates a CurseForge API client. The API key is
// optional: when none is configured the client still issues requests, which
// works against keyless read-only mirrors/caches. Against the
// official api.curseforge.com a missing key yields an authenticated 403 with
// a hint (see do()).
func NewCurseForgeClient(cfg *config.Config) *CurseForgeClient {
	ua := cfg.UserAgent
	if ua == "" {
		ua = config.DefaultUserAgent
	}
	base := cfg.CurseForgeAPIBase
	if base == "" {
		base = config.DefaultCurseForgeBase
	}
	return &CurseForgeClient{
		base:      base,
		http:      netx.NewClient(30 * time.Second),
		apiKey:    cfg.CurseForgeAPIKey,
		userAgent: ua,
	}
}

func (c *CurseForgeClient) Name() string { return PlatformCurseForge }

// do performs a GET request and decodes JSON into out.
func (c *CurseForgeClient) do(ctx context.Context, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		var apiErr struct {
			ErrorCode    int    `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.ErrorMessage != "" {
			return fmt.Errorf("curseforge API %s: %s", resp.Status, apiErr.ErrorMessage)
		}
		// The official API rejects unauthenticated requests with 403; the bare
		// message is confusing, so point the user at a fix.
		if resp.StatusCode == http.StatusForbidden && c.apiKey == "" && c.base == config.DefaultCurseForgeBase {
			return fmt.Errorf("curseforge API %s: missing API key. Set CURSEFORGE_API_KEY (or \"curseforge_api_key\" in ~/.cubehaul/config.json) to use the official API", resp.Status)
		}
		return fmt.Errorf("curseforge API %d %s: %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// cfMod is the mod object shared by search and detail responses.
type cfMod struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Summary       string `json:"summary"`
	DownloadCount int64  `json:"downloadCount"`
	ThumbsUpCount int64  `json:"thumbsUpCount"`
	DateModified  string `json:"dateModified"`
	Links         struct {
		WebsiteURL string `json:"websiteUrl"`
	} `json:"links"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

func (m cfMod) project() Project {
	author := ""
	if len(m.Authors) > 0 {
		author = m.Authors[0].Name
	}
	catNames := make([]string, 0, len(m.Categories))
	for _, c := range m.Categories {
		catNames = append(catNames, c.Name)
	}
	return Project{
		Platform:    PlatformCurseForge,
		ID:          strconv.Itoa(m.ID),
		Slug:        m.Slug,
		Title:       m.Name,
		Description: m.Summary,
		Author:      author,
		Downloads:   m.DownloadCount,
		Follows:     m.ThumbsUpCount,
		Categories:  catNames,
		URL:         m.Links.WebsiteURL,
		UpdatedAt:   m.DateModified,
	}
}

func (c *CurseForgeClient) Search(ctx context.Context, o SearchOptions) ([]Project, error) {
	// modId is not a search parameter: fetch the mod directly.
	if o.ModID > 0 {
		p, err := c.GetProject(ctx, strconv.Itoa(o.ModID))
		if err != nil {
			return nil, err
		}
		return []Project{*p}, nil
	}

	q := url.Values{}
	q.Set("gameId", strconv.Itoa(curseForgeGameID))
	if o.Query != "" {
		q.Set("searchFilter", o.Query)
	}

	classID := o.ClassID
	if classID == 0 && o.ProjectType != "" {
		id, ok := curseForgeClassIDs[strings.ToLower(o.ProjectType)]
		if !ok {
			return nil, fmt.Errorf("unsupported project type %q for curseforge (supported: mod, modpack, resourcepack, shader, datapack, plugin, customization, addons, worlds)", o.ProjectType)
		}
		classID = id
	}
	if classID == 0 {
		classID = 6 // default: Minecraft mods
	}
	q.Set("classId", strconv.Itoa(classID))

	if o.CategoryID > 0 {
		q.Set("categoryId", strconv.Itoa(o.CategoryID))
	} else if len(o.Categories) > 0 {
		id, err := c.resolveCategory(ctx, o.Categories[0], classID)
		if err != nil {
			return nil, err
		}
		q.Set("categoryId", strconv.Itoa(id))
	}
	if len(o.GameVersions) > 0 {
		q.Set("gameVersion", o.GameVersions[0]) // CurseForge accepts a single gameVersion
	}
	if len(o.Loaders) > 0 {
		id, ok := curseForgeLoaderTypes[strings.ToLower(o.Loaders[0])]
		if !ok {
			return nil, fmt.Errorf("unsupported loader %q for curseforge (supported: forge, cauldron, liteloader, fabric, quilt, neoforge)", o.Loaders[0])
		}
		q.Set("modLoaderType", strconv.Itoa(id))
	}
	if o.GameVersionTypeID > 0 {
		q.Set("gameVersionTypeId", strconv.Itoa(o.GameVersionTypeID))
	}
	if o.Slug != "" {
		q.Set("slug", o.Slug)
	}
	field, order, err := cfSortQuery(o)
	if err != nil {
		return nil, err
	}
	if field > 0 {
		q.Set("sortField", strconv.Itoa(field))
	}
	if order != "" {
		q.Set("sortOrder", order)
	}
	pageSize := 10
	if o.Limit > 0 {
		pageSize = min(o.Limit, 50)
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	if o.Offset > 0 {
		q.Set("index", strconv.Itoa(o.Offset))
	}
	for _, rp := range o.RawParams {
		k, v, ok := strings.Cut(rp, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --raw-param %q (expected key=value)", rp)
		}
		q.Set(k, v)
	}

	var resp struct {
		Data []cfMod `json:"data"`
	}
	if err := c.do(ctx, "/mods/search", q, &resp); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(resp.Data))
	for _, m := range resp.Data {
		projects = append(projects, m.project())
	}
	return projects, nil
}

func (c *CurseForgeClient) GetProject(ctx context.Context, id string) (*Project, error) {
	modID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("curseforge project id must be numeric, got %q", id)
	}
	var resp struct {
		Data cfMod `json:"data"`
	}
	if err := c.do(ctx, "/mods/"+strconv.Itoa(modID), nil, &resp); err != nil {
		return nil, err
	}
	p := resp.Data.project()
	return &p, nil
}

// cfFile is a file object in /mods/{modId}/files responses.
type cfFile struct {
	ID           int      `json:"id"`
	ModID        int      `json:"modId"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	FileDate     string   `json:"fileDate"`
	FileLength   int64    `json:"fileLength"`
	FileStatus   int      `json:"fileStatus"`
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
	Changelog    string   `json:"changelog"`
}

// cfDownloadURL returns the download URL for a file, falling back to the
// ForgeCDN link pattern (fileId split as id/1000 / id%1000) when the API
// returns no direct URL.
func cfDownloadURL(f cfFile) string {
	if f.DownloadURL != "" {
		return f.DownloadURL
	}
	esc := url.PathEscape(f.FileName)
	return fmt.Sprintf("https://mediafilez.forgecdn.net/files/%d/%03d/%s", f.ID/1000, f.ID%1000, esc)
}

// inferCFLoaders extracts loader names from a file's gameVersions list, where
// CurseForge mixes loaders ("Fabric") and game versions ("1.20.1").
func inferCFLoaders(gameVersions []string) []string {
	var out []string
	for _, gv := range gameVersions {
		for _, l := range cfLoaderNames {
			if strings.EqualFold(gv, l) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

func (c *CurseForgeClient) ListVersions(ctx context.Context, projectID string, loaders, gameVersions []string) ([]Version, error) {
	modID, err := strconv.Atoi(projectID)
	if err != nil {
		return nil, fmt.Errorf("curseforge project id must be numeric, got %q", projectID)
	}
	q := url.Values{}
	q.Set("pageSize", "50")

	var resp struct {
		Data []cfFile `json:"data"`
	}
	if err := c.do(ctx, "/mods/"+strconv.Itoa(modID)+"/files", q, &resp); err != nil {
		return nil, err
	}
	versions := make([]Version, 0, len(resp.Data))
	for _, f := range resp.Data {
		// CurseForge fileStatus enum: 1=Processing, 4=Approved. Only Approved
		// files are downloadable; skip processing/under-review/removed/etc.
		if f.FileStatus != curseForgeFileStatusApproved {
			continue
		}
		versions = append(versions, Version{
			ID:            strconv.Itoa(f.ID),
			ProjectID:     strconv.Itoa(f.ModID),
			Name:          f.DisplayName,
			VersionNumber: f.DisplayName,
			DatePublished: f.FileDate,
			GameVersions:  f.GameVersions,
			Loaders:       inferCFLoaders(f.GameVersions),
			Changelog:     f.Changelog,
			Files: []File{{
				ID:       strconv.Itoa(f.ID),
				Filename: f.FileName,
				URL:      cfDownloadURL(f),
				Size:     f.FileLength,
				Primary:  true,
			}},
		})
	}
	sortVersionsByDate(versions)

	// CurseForge cannot filter files by loader/version server-side; filter here.
	if len(loaders) > 0 || len(gameVersions) > 0 {
		filtered := versions[:0]
		for _, v := range versions {
			if len(loaders) > 0 && !anyFold(v.Loaders, loaders) {
				continue
			}
			if len(gameVersions) > 0 && !anyExact(v.GameVersions, gameVersions) {
				continue
			}
			filtered = append(filtered, v)
		}
		versions = filtered
	}
	return versions, nil
}

type cfCategory struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	ClassID          int    `json:"classId"`
	ParentCategoryID int    `json:"parentCategoryId"`
	IsClass          bool   `json:"isClass"`
}

// Categories lists the Minecraft category tree. The endpoint is accessible
// without an API key.
func (c *CurseForgeClient) Categories(ctx context.Context, classID int) ([]Category, error) {
	q := url.Values{}
	q.Set("gameId", strconv.Itoa(curseForgeGameID))
	var resp struct {
		Data []cfCategory `json:"data"`
	}
	if err := c.do(ctx, "/categories", q, &resp); err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(resp.Data))
	for _, cc := range resp.Data {
		if classID > 0 {
			if cc.IsClass {
				if cc.ID != classID {
					continue
				}
			} else if cc.ClassID != classID {
				continue
			}
		}
		cats = append(cats, Category{
			ID:       strconv.Itoa(cc.ID),
			Name:     cc.Name,
			Slug:     cc.Slug,
			ClassID:  cc.ClassID,
			ParentID: cc.ParentCategoryID,
			IsClass:  cc.IsClass,
		})
	}
	return cats, nil
}

// resolveCategory finds a category ID by name or slug, preferring a match
// within the current class.
func (c *CurseForgeClient) resolveCategory(ctx context.Context, name string, classID int) (int, error) {
	cats, err := c.Categories(ctx, classID)
	if err != nil {
		return 0, err
	}
	var matches []Category
	for _, cat := range cats {
		if strings.EqualFold(cat.Name, name) || strings.EqualFold(cat.Slug, name) {
			matches = append(matches, cat)
		}
	}
	if len(matches) == 0 {
		return 0, fmt.Errorf("category %q not found for class %d (see `cubehaul categories curseforge` for the full list)", name, classID)
	}
	for _, m := range matches {
		if m.ClassID == classID || m.IsClass {
			id, err := strconv.Atoi(m.ID)
			if err == nil {
				return id, nil
			}
		}
	}
	id, err := strconv.Atoi(matches[0].ID)
	if err != nil {
		return 0, fmt.Errorf("internal error: bad category id %q", matches[0].ID)
	}
	return id, nil
}
