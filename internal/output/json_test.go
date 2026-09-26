package output

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"cubehaul/internal/platform"
)

// jsonTags returns the JSON names declared by v's struct type; v may be a
// pointer or a slice of it.
func jsonTags(t *testing.T, v any) []string {
	t.Helper()
	typ := reflect.TypeOf(v)
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		t.Fatalf("%s is not a struct", typ)
	}
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// TestFieldListsMatchStructs guards the --json contract: the declared lists are
// what --json=field validates against and what docs/json-fields.md documents, so
// adding a struct field without updating its list must fail here instead of
// silently disappearing from the output.
func TestFieldListsMatchStructs(t *testing.T) {
	cases := []struct {
		resource string
		declared []string
		sample   any
	}{
		{"project", projectFields, platform.Project{}},
		{"version", versionFields, platform.Version{}},
		{"category", categoryFields, platform.Category{}},
		{"download", downloadFields, Download{}},
		{"config", configFields, ConfigRow{}},
		{"target", targetFields, Target{}},
	}
	for _, c := range cases {
		if want := jsonTags(t, c.sample); !reflect.DeepEqual(c.declared, want) {
			t.Errorf("%s fields = %v, want the struct tags %v (order included)", c.resource, c.declared, want)
		}
	}
}

func TestParseSelector(t *testing.T) {
	cases := []struct {
		raw  string
		want Selector
	}{
		{"", Selector{}},
		{"all", Selector{On: true, All: true}},
		{"id,title", Selector{On: true, Fields: []string{"id", "title"}}},
		{" id , title ", Selector{On: true, Fields: []string{"id", "title"}}},
		{"id,,title,", Selector{On: true, Fields: []string{"id", "title"}}},
	}
	for _, c := range cases {
		if got := ParseSelector(c.raw); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseSelector(%q) = %+v, want %+v", c.raw, got, c.want)
		}
	}
}

func TestNewFormat(t *testing.T) {
	if f := NewFormat("", ""); f.JSON.On {
		t.Error("no flags should leave JSON off")
	}
	// A jq expression implies JSON, so --jq works on its own.
	if f := NewFormat("", ".[].title"); !f.JSON.On || !f.JSON.All {
		t.Errorf("--jq should imply --json with every field, got %+v", f.JSON)
	}
	if f := NewFormat("id,title", "  "); !reflect.DeepEqual(f.JSON.Fields, []string{"id", "title"}) || f.JQ != "" {
		t.Errorf("field selection lost: %+v", f)
	}
}

func TestRecordSelectsAndOrdersFields(t *testing.T) {
	f := NewFormat("title,id", "")
	rec, err := f.record(platform.Project{ID: "AANobbMI", Title: "Sodium", Slug: "sodium"}, projectFields)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if string(rec) != `{"title":"Sodium","id":"AANobbMI"}` {
		t.Errorf("record = %s, want the requested order", rec)
	}

	f = NewFormat("id,nope", "")
	if _, err := f.record(platform.Project{}, projectFields); err == nil {
		t.Fatal("want an error for an unknown field")
	} else if !strings.Contains(err.Error(), "valid fields:") {
		t.Errorf("error %q should list the valid fields", err)
	}

	// A field the value leaves out (omitempty) is absent rather than an error.
	f = NewFormat("id,class_id", "")
	rec, err = f.record(platform.Category{ID: "6", Name: "Mods"}, categoryFields)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if string(rec) != `{"id":"6"}` {
		t.Errorf("record = %s, want only the fields actually present", rec)
	}
}

// captureStdout runs fn with stdout redirected, so the renderers can be tested
// without a subprocess.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	runErr := fn()
	os.Stdout = saved
	w.Close()
	out, _ := io.ReadAll(r)
	r.Close()
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	return string(out)
}

func TestEmitListHonoursSelection(t *testing.T) {
	got := captureStdout(t, func() error {
		return emitList(NewFormat("id,title", ""), []platform.Project{{ID: "1", Title: "A"}}, projectFields)
	})
	want := "[\n  {\n    \"id\": \"1\",\n    \"title\": \"A\"\n  }\n]\n"
	if got != want {
		t.Errorf("emitList = %q, want %q", got, want)
	}

	// No selection: every declared field, still in declared order.
	got = captureStdout(t, func() error {
		return emitList(NewFormat("all", ""), []platform.Project{{ID: "1", Title: "A"}}, projectFields)
	})
	if !strings.HasPrefix(got, "[\n  {\n    \"platform\": \"\",\n    \"id\": \"1\",") {
		t.Errorf("all-fields output does not start with the declared order: %q", got)
	}
}

func TestRunJQ(t *testing.T) {
	doc := []byte(`[{"title":"A","downloads":2},{"title":"B","downloads":3}]`)

	// Strings print raw, one per line.
	if got := captureStdout(t, func() error { return runJQ(doc, ".[].title") }); got != "A\nB\n" {
		t.Errorf(".[].title = %q, want A\\nB\\n", got)
	}
	// Everything else prints as JSON.
	if got := captureStdout(t, func() error { return runJQ(doc, "map(.downloads) | add") }); got != "5\n" {
		t.Errorf("add = %q, want 5", got)
	}
	if got := captureStdout(t, func() error { return runJQ(doc, ".[0]") }); got != `{"downloads":2,"title":"A"}`+"\n" {
		t.Errorf(".[0] = %q", got)
	}
	// select() and a runtime error must surface, not be swallowed.
	if got := captureStdout(t, func() error { return runJQ(doc, `.[] | select(.downloads > 2) | .title`) }); got != "B\n" {
		t.Errorf("select = %q, want B", got)
	}
	if err := runJQ(doc, ".["); err == nil {
		t.Error("want a parse error for a broken expression")
	}
}

func TestDownloadResultFields(t *testing.T) {
	d := Download{Platform: "modrinth", Project: "sodium", Version: "mc1.21-0.6.0", VersionID: "v1", File: "sodium.jar", URL: "https://cdn/x.jar", Path: "/tmp/sodium.jar", Size: 42}
	got := captureStdout(t, func() error { return DownloadResult(d, NewFormat("path,size", "")) })
	want := "{\n  \"path\": \"/tmp/sodium.jar\",\n  \"size\": 42\n}\n"
	if got != want {
		t.Errorf("DownloadResult = %q, want %q", got, want)
	}
}
