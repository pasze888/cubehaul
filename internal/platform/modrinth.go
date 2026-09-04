package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cubehaul/internal/config"
	"cubehaul/internal/netx"
)

// validModrinthFacetTypes are the facet types accepted by GET /search.
var validModrinthFacetTypes = map[string]bool{
	"project_type": true, "all_project_types": true,
	"categories": true, "versions": true,
	"open_source": true, "environment": true,
	"client_side": true, "server_side": true, // deprecated, still accepted
	"title": true, "author": true, "license": true, "project_id": true,
	"follows": true, "downloads": true,
	"created_timestamp": true, "modified_timestamp": true,
}

// modrinthSortIndexes maps --sort values to the `index` parameter.
var modrinthSortIndexes = map[string]string{
	"relevance": "relevance",
	"downloads": "downloads",
	"follows":   "follows",
	"newest":    "newest",
	"updated":   "updated",
}

// facetRe matches "type op value" where op is one of : != >= <= > < =.
var facetRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(!=|>=|<=|>|<|:|=)(.+)$`)

// splitFacet parses a raw facet string of the form "type op value" and
// returns the type plus the original string. The live Modrinth parser
// accepts ":" or "=" for equality and !, !=, >=, <=, >, < directly after
// the type for comparisons ("downloads>=100"); ":" is itself the equality
// operator and cannot be combined with a comparison.
func splitFacet(f string) (typ, normalized string, err error) {
	m := facetRe.FindStringSubmatch(f)
	if m == nil {
		return "", "", fmt.Errorf("invalid facet %q (expected form: type op value, e.g. categories:adventure or downloads>=100)", f)
	}
	typ, op, val := m[1], m[2], m[3]
	if op == ":" || op == "=" {
		if strings.ContainsAny(val[:1], "!<>= ") {
			return "", "", fmt.Errorf("invalid facet %q: comparison operators follow the type directly, write %s%s%s without ':'", f, typ, op, val)
		}
	}
	return typ, f, nil
}

// ModrinthClient talks to api.modrinth.com/v2.
type ModrinthClient struct {
	base      string
	http      *http.Client
	userAgent string
}

// NewModrinthClient creates a Modrinth API client. Modrinth requires no
// authentication but demands a User-Agent on every request.
func NewModrinthClient(cfg *config.Config) *ModrinthClient {
	ua := cfg.UserAgent
	if ua == "" {
		ua = config.DefaultUserAgent
	}
	base := cfg.ModrinthAPIBase
	if base == "" {
		base = config.DefaultModrinthBase
	}
	return &ModrinthClient{
		base:      base,
		http:      netx.NewClient(30 * time.Second),
		userAgent: ua,
	}
}

func (c *ModrinthClient) Name() string { return PlatformModrinth }

// do performs a GET request and decodes JSON into out.
func (c *ModrinthClient) do(ctx context.Context, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := netx.DoWithRetry(ctx, c.http, req, netx.RetryOptions{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		var apiErr struct {
			Error       string `json:"error"`
			Description string `json:"description"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Description != "" {
			return fmt.Errorf("modrinth API %s: %s", resp.Status, apiErr.Description)
		}
		return fmt.Errorf("modrinth API %d %s: %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// buildModrinthFacets converts SearchOptions into facet groups.
// Groups are ANDed; entries within a group are ORed.
func buildModrinthFacets(o SearchOptions) ([][]string, error) {
	groups := map[string][]string{}
	add := func(f string) error {
		typ, norm, err := splitFacet(f)
		if err != nil {
			return err
		}
		if !validModrinthFacetTypes[typ] {
			return fmt.Errorf("unknown modrinth facet type %q (supported: project_type, all_project_types, categories, versions, open_source, environment, author, license, downloads, follows, ...)", typ)
		}
		groups[typ] = append(groups[typ], norm)
		return nil
	}

	if o.ProjectType != "" {
		if err := add("project_type:" + o.ProjectType); err != nil {
			return nil, err
		}
	}
	for _, cat := range o.Categories {
		if err := add("categories:" + cat); err != nil {
			return nil, err
		}
	}
	for _, l := range o.Loaders { // loaders are lumped into categories in search
		if err := add("categories:" + l); err != nil {
			return nil, err
		}
	}
	for _, v := range o.GameVersions {
		if err := add("versions:" + v); err != nil {
			return nil, err
		}
	}
	if o.Environment != "" {
		if err := add("environment:" + o.Environment); err != nil {
			return nil, err
		}
	}
	if o.Author != "" {
		if err := add("author:" + o.Author); err != nil {
			return nil, err
		}
	}
	if o.License != "" {
		if err := add("license:" + o.License); err != nil {
			return nil, err
		}
	}
	if o.OpenSource != nil {
		v := "false"
		if *o.OpenSource {
			v = "true"
		}
		if err := add("open_source:" + v); err != nil {
			return nil, err
		}
	}
	for _, f := range o.Facets {
		if err := add(f); err != nil {
			return nil, err
		}
	}

	var extra [][]string
	if o.FacetsJSON != "" {
		if err := json.Unmarshal([]byte(o.FacetsJSON), &extra); err != nil {
			return nil, fmt.Errorf("invalid --facets-json: %w", err)
		}
		for _, g := range extra {
			for _, f := range g {
				typ, _, err := splitFacet(f)
				if err != nil {
					return nil, err
				}
				if !validModrinthFacetTypes[typ] {
					return nil, fmt.Errorf("unknown modrinth facet type %q in --facets-json", typ)
				}
			}
		}
	}

	if len(groups) == 0 && len(extra) == 0 {
		return nil, nil
	}
	types := make([]string, 0, len(groups))
	for t := range groups {
		types = append(types, t)
	}
	sort.Strings(types)
	facets := make([][]string, 0, len(types)+len(extra))
	for _, t := range types {
		facets = append(facets, groups[t])
	}
	facets = append(facets, extra...)
	return facets, nil
}

type modrinthSearchResponse struct {
	Hits []struct {
		ProjectID    string   `json:"project_id"`
		ProjectType  string   `json:"project_type"`
		Slug         string   `json:"slug"`
		Author       string   `json:"author"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Categories   []string `json:"categories"`
		Downloads    int64    `json:"downloads"`
		Follows      int64    `json:"follows"`
		License      string   `json:"license"`
		DateModified string   `json:"date_modified"`
	} `json:"hits"`
	TotalHits int `json:"total_hits"`
}

func (c *ModrinthClient) Search(ctx context.Context, o SearchOptions) ([]Project, error) {
	q := url.Values{}
	if o.Query != "" {
		q.Set("query", o.Query)
	}
	facets, err := buildModrinthFacets(o)
	if err != nil {
		return nil, err
	}
	if facets != nil {
		b, _ := json.Marshal(facets)
		q.Set("facets", string(b))
	}
	if o.Sort != "" {
		idx, ok := modrinthSortIndexes[o.Sort]
		if !ok {
			return nil, fmt.Errorf("unsupported sort %q for modrinth (supported: relevance, downloads, follows, newest, updated)", o.Sort)
		}
		q.Set("index", idx)
	}
	if o.Limit > 0 {
		limit, notice := clampSearchLimit(PlatformModrinth, o.Limit, ModrinthMaxSearchLimit)
		if notice != "" {
			fmt.Fprint(os.Stderr, notice)
		}
		q.Set("limit", strconv.Itoa(limit))
	}
	if o.Offset > 0 {
		q.Set("offset", strconv.Itoa(o.Offset))
	}

	var resp modrinthSearchResponse
	if err := c.do(ctx, "/search", q, &resp); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		projects = append(projects, Project{
			Platform:    PlatformModrinth,
			ID:          h.ProjectID,
			Slug:        h.Slug,
			Title:       h.Title,
			Description: h.Description,
			Author:      h.Author,
			Downloads:   h.Downloads,
			Follows:     h.Follows,
			Categories:  h.Categories,
			License:     h.License,
			URL:         "https://modrinth.com/" + h.ProjectType + "/" + h.Slug,
			UpdatedAt:   h.DateModified,
		})
	}
	return projects, nil
}

type modrinthProject struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ProjectType string   `json:"project_type"`
	Author      string   `json:"author"`
	Downloads   int64    `json:"downloads"`
	Followers   int64    `json:"followers"`
	Categories  []string `json:"categories"`
	License     struct {
		ID string `json:"id"`
	} `json:"license"`
	Updated string `json:"updated"`
}

func (c *ModrinthClient) GetProject(ctx context.Context, id string) (*Project, error) {
	var p modrinthProject
	if err := c.do(ctx, "/project/"+url.PathEscape(id), nil, &p); err != nil {
		return nil, err
	}
	author := p.Author
	if author == "" { // current API returns author: null on /project; fall back to members
		if members, err := c.members(ctx, p.ID); err == nil && len(members) > 0 {
			author = members[0]
		}
	}
	return &Project{
		Platform:    PlatformModrinth,
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Description: p.Description,
		Author:      author,
		Downloads:   p.Downloads,
		Follows:     p.Followers,
		Categories:  p.Categories,
		License:     p.License.ID,
		URL:         "https://modrinth.com/" + p.ProjectType + "/" + p.Slug,
		UpdatedAt:   p.Updated,
	}, nil
}

// members returns the usernames of a project's team, in display order.
func (c *ModrinthClient) members(ctx context.Context, projectID string) ([]string, error) {
	var raw []struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := c.do(ctx, "/project/"+url.PathEscape(projectID)+"/members", nil, &raw); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(raw))
	for _, m := range raw {
		if m.User.Username != "" {
			names = append(names, m.User.Username)
		}
	}
	return names, nil
}

type modrinthVersion struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	DatePublished string   `json:"date_published"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	Changelog     string   `json:"changelog"`
	Files         []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		Primary  bool   `json:"primary"`
	} `json:"files"`
}

func (c *ModrinthClient) ListVersions(ctx context.Context, projectID string, loaders, gameVersions []string) ([]Version, error) {
	q := url.Values{}
	if len(loaders) > 0 {
		b, _ := json.Marshal(loaders)
		q.Set("loaders", string(b))
	}
	if len(gameVersions) > 0 {
		b, _ := json.Marshal(gameVersions)
		q.Set("game_versions", string(b))
	}

	var raw []modrinthVersion
	if err := c.do(ctx, "/project/"+url.PathEscape(projectID)+"/version", q, &raw); err != nil {
		return nil, err
	}
	versions := make([]Version, 0, len(raw))
	for _, v := range raw {
		ver := Version{
			ID:            v.ID,
			ProjectID:     v.ProjectID,
			Name:          v.Name,
			VersionNumber: v.VersionNumber,
			DatePublished: v.DatePublished,
			GameVersions:  v.GameVersions,
			Loaders:       v.Loaders,
			Changelog:     v.Changelog,
			Files:         make([]File, 0, len(v.Files)),
		}
		for _, f := range v.Files {
			ver.Files = append(ver.Files, File{
				ID:       f.Filename,
				Filename: f.Filename,
				URL:      f.URL,
				Size:     f.Size,
				Primary:  f.Primary,
			})
		}
		versions = append(versions, ver)
	}
	sortVersionsByDate(versions)
	return versions, nil
}

type modrinthTag struct {
	ProjectType string `json:"project_type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *ModrinthClient) Categories(ctx context.Context, classID int) ([]Category, error) {
	var tags []modrinthTag
	if err := c.do(ctx, "/tag/category", nil, &tags); err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(tags))
	for _, t := range tags {
		cats = append(cats, Category{
			ID:   t.Name,
			Name: t.Name,
			Slug: strings.ToLower(strings.ReplaceAll(t.Name, " ", "-")),
		})
	}
	return cats, nil
}
