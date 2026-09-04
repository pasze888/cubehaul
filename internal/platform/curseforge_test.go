package platform

import "testing"

// TestCurseForgeLoaderTypesWithinEnum guards the modLoaderType mapping against
// CurseForge's enum, which only defines Any=0 through NeoForge=6. Ids above 6
// are not merely ignored: the API rejects the whole request with
// "The value 'N' is invalid", which is how neoforge=8 used to break every
// `curseforge search --loader neoforge`.
func TestCurseForgeLoaderTypesWithinEnum(t *testing.T) {
	const maxValid = 6
	for name, id := range curseForgeLoaderTypes {
		if id < 1 || id > maxValid {
			t.Errorf("curseForgeLoaderTypes[%q] = %d, want 1..%d", name, id, maxValid)
		}
	}
}

// TestCfSortQuery pins the sort resolution, including the relevancy default
// that only applies to free-text queries and the desc direction it needs to
// rank best-match-first.
func TestCfSortQuery(t *testing.T) {
	tests := []struct {
		name      string
		o         SearchOptions
		wantField int
		wantOrder string
	}{
		{"no query no sort sends nothing", SearchOptions{}, 0, ""},
		{"query defaults to relevancy desc", SearchOptions{Query: "sodium"}, curseForgeSortRelevancy, "desc"},
		{"filter-only search keeps api default", SearchOptions{Categories: []string{"technology"}}, 0, ""},
		{"explicit sort wins over default", SearchOptions{Query: "sodium", Sort: "downloads"}, curseForgeSortDownloads, "desc"},
		{"downloads gets desc without sort-order", SearchOptions{Sort: "downloads"}, curseForgeSortDownloads, "desc"},
		{"updated gets desc", SearchOptions{Sort: "updated"}, curseForgeSortLastUpdated, "desc"},
		{"popularity gets desc", SearchOptions{Sort: "popularity"}, curseForgeSortPopularity, "desc"},
		{"name gets asc", SearchOptions{Sort: "name"}, curseForgeSortName, "asc"},
		{"author gets asc", SearchOptions{Sort: "author"}, curseForgeSortAuthor, "asc"},
		{"featured keeps server direction", SearchOptions{Sort: "featured"}, curseForgeSortFeatured, ""},
		{"category keeps server direction", SearchOptions{Sort: "category"}, curseForgeSortCategory, ""},
		{"game-version keeps server direction", SearchOptions{Sort: "game-version"}, curseForgeSortGameVersion, ""},
		{"explicit relevancy also gets desc", SearchOptions{Query: "jei", Sort: "relevancy"}, curseForgeSortRelevancy, "desc"},
		{"user sort-order overrides desc", SearchOptions{Query: "sodium", Sort: "relevancy", SortOrder: "asc"}, curseForgeSortRelevancy, "asc"},
		{"user sort-order overrides asc default", SearchOptions{Sort: "name", SortOrder: "desc"}, curseForgeSortName, "desc"},
		{"sort-order applies to other fields", SearchOptions{Query: "sodium", Sort: "downloads", SortOrder: "desc"}, curseForgeSortDownloads, "desc"},
		{"case insensitive sort", SearchOptions{Sort: "Popularity"}, curseForgeSortPopularity, "desc"},
		{"sort-order alone orders nothing", SearchOptions{SortOrder: "desc"}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, order, err := cfSortQuery(tt.o)
			if err != nil {
				t.Fatalf("cfSortQuery(%+v) unexpected error: %v", tt.o, err)
			}
			if field != tt.wantField || order != tt.wantOrder {
				t.Errorf("cfSortQuery(%+v) = (%d, %q), want (%d, %q)",
					tt.o, field, order, tt.wantField, tt.wantOrder)
			}
		})
	}
}

func TestCfSortQueryRejectsBadInput(t *testing.T) {
	if _, _, err := cfSortQuery(SearchOptions{Sort: "random"}); err == nil {
		t.Error("unknown sort should error")
	}
	if _, _, err := cfSortQuery(SearchOptions{SortOrder: "garbage"}); err == nil {
		t.Error("unknown sort-order should error")
	}
}

func TestCurseForgeLoaderTypeNames(t *testing.T) {
	want := map[string]int{
		"forge":      1,
		"cauldron":   2,
		"liteloader": 3,
		"fabric":     4,
		"quilt":      5,
		"neoforge":   6,
	}
	for name, id := range want {
		if got := curseForgeLoaderTypes[name]; got != id {
			t.Errorf("loader %q mapped to %d, want %d", name, got, id)
		}
	}
	if len(curseForgeLoaderTypes) != len(want) {
		t.Errorf("curseForgeLoaderTypes has %d entries, want %d (a loader was added or dropped)",
			len(curseForgeLoaderTypes), len(want))
	}
	// Rift is not part of the enum; mapping it would 400 at the API.
	if _, ok := curseForgeLoaderTypes["rift"]; ok {
		t.Error("rift must not be mapped: CurseForge has no such modLoaderType")
	}
}
