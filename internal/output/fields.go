package output

// Field lists are the JSON contract of every resource, in output order. They are
// declared here instead of derived from the Go structs at run time so the
// contract is greppable, documented in docs/json-fields.md and guarded by
// TestFieldListsMatchStructs, which fails when a struct field is added without
// updating its list.
var (
	projectFields = []string{
		"platform", "id", "slug", "title", "description", "author",
		"downloads", "follows", "categories", "license", "url", "updated_at",
	}
	versionFields = []string{
		"id", "project_id", "name", "version_number", "date_published",
		"game_versions", "loaders", "files", "changelog",
	}
	categoryFields = []string{"id", "name", "slug", "class_id", "parent_id", "is_class"}
	downloadFields = []string{"platform", "project", "version", "version_id", "file", "url", "path", "size"}
	configFields   = []string{"key", "value", "source", "secret"}
	targetFields   = []string{"file", "minecraft_version", "minecraft_version_source", "loaders"}
)

// Download is the machine-readable result of a successful "download".
type Download struct {
	Platform  string `json:"platform"`
	Project   string `json:"project"`
	Version   string `json:"version"`
	VersionID string `json:"version_id"`
	File      string `json:"file"`
	URL       string `json:"url"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
}

// DownloadResult prints the result of a download as JSON.
func DownloadResult(d Download, f Format) error {
	return emitOne(f, d, downloadFields)
}

// Target is the mod development target detected in a gradle.properties (see the
// "target" command). MinecraftVersionSource names the property the version came
// from, so a value derived from a version range is visible as such.
type Target struct {
	File                   string   `json:"file"`
	MinecraftVersion       string   `json:"minecraft_version"`
	MinecraftVersionSource string   `json:"minecraft_version_source"`
	Loaders                []string `json:"loaders"`
}
