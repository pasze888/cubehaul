package platform

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// queryRecorder safely stores the last request's query for assertions; the
// handler goroutine writes while the test reads after ListVersions returns.
type queryRecorder struct {
	mu  sync.Mutex
	val url.Values
}

func (qr *queryRecorder) set(q url.Values) {
	qr.mu.Lock()
	qr.val = q
	qr.mu.Unlock()
}

func (qr *queryRecorder) get() url.Values {
	qr.mu.Lock()
	defer qr.mu.Unlock()
	return qr.val
}

// writeFilesPage emits one /mods/{id}/files response: n items of synthetic
// Approved files tagged 1.21.1+Forge, with the pagination block the client
// relies on for its loop.
func writeFilesPage(w http.ResponseWriter, total, pageSize, index int) {
	n := pageSize
	if index+n > total {
		n = total - index
	}
	if n < 0 {
		n = 0
	}
	var sb strings.Builder
	sb.WriteString(`{"data":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		id := 900000 + index + i
		fmt.Fprintf(&sb,
			`{"id":%d,"modId":238222,"displayName":"file %d","fileName":"f%d.jar",`+
				`"fileDate":"2024-01-01T00:00:00Z","fileLength":1,"fileStatus":4,`+
				`"downloadUrl":"","gameVersions":["1.21.1","Forge"],"changelog":""}`,
			id, id, id)
	}
	fmt.Fprintf(&sb, `],"pagination":{"index":%d,"pageSize":%d,"resultCount":%d,"totalCount":%d}}`,
		index, pageSize, n, total)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, sb.String())
}

// newFilesServer serves synthetic pages for /mods/{id}/files, honouring the
// index/pageSize the client asked for, and records the last query.
func newFilesServer(t *testing.T, total int) (*httptest.Server, *queryRecorder) {
	t.Helper()
	rec := &queryRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		rec.set(q)
		index, _ := strconv.Atoi(q.Get("index"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		writeFilesPage(w, total, pageSize, index)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func newTestCFClient(srv *httptest.Server) *CurseForgeClient {
	return &CurseForgeClient{base: srv.URL, http: srv.Client(), userAgent: "test"}
}

func TestListVersionsPagesThroughAllFiles(t *testing.T) {
	// 250 matching files: page 1 (200) is not enough; the loop must fetch
	// page 2 using index=200 and stop at pagination.totalCount.
	srv, rec := newFilesServer(t, 250)
	c := newTestCFClient(srv)
	versions, err := c.ListVersions(context.Background(), "238222", nil, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 250 {
		t.Errorf("versions = %d, want 250", len(versions))
	}
	if got := rec.get().Get("pageSize"); got != "200" {
		t.Errorf("pageSize = %q, want 200", got)
	}
}

func TestListVersionsPushesDownSingleValuedFilters(t *testing.T) {
	srv, rec := newFilesServer(t, 250)
	c := newTestCFClient(srv)
	versions, err := c.ListVersions(context.Background(), "238222", []string{"forge"}, []string{"1.21.1"})
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	q := rec.get()
	if got := q.Get("modLoaderType"); got != "1" { // forge → modLoaderType 1
		t.Errorf("modLoaderType = %q, want 1", got)
	}
	if got := q.Get("gameVersion"); got != "1.21.1" {
		t.Errorf("gameVersion = %q, want 1.21.1", got)
	}
	if len(versions) != 250 {
		t.Errorf("versions = %d, want 250", len(versions))
	}
}

func TestListVersionsSkipsPushdownForMultiValuedFilters(t *testing.T) {
	srv, rec := newFilesServer(t, 250)
	c := newTestCFClient(srv)
	versions, err := c.ListVersions(context.Background(), "238222",
		[]string{"forge", "fabric"}, []string{"1.21.1", "1.21"})
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	q := rec.get()
	if q.Has("modLoaderType") || q.Has("gameVersion") {
		t.Errorf("multi-valued filters must not be pushed down, got %v", q)
	}
	// All synthetic files are 1.21.1+Forge, so both filters keep them.
	if len(versions) != 250 {
		t.Errorf("versions = %d, want 250", len(versions))
	}
}

func TestListVersionsUnknownLoaderNotPushedDown(t *testing.T) {
	srv, rec := newFilesServer(t, 250)
	c := newTestCFClient(srv)
	// "rift" has no modLoaderType enum entry: nothing to push down, and the
	// client-side filter rejects every synthetic Forge file.
	versions, err := c.ListVersions(context.Background(), "238222", []string{"rift"}, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if rec.get().Has("modLoaderType") {
		t.Error("unknown loader must not be pushed down")
	}
	if len(versions) != 0 {
		t.Errorf("versions = %d, want 0", len(versions))
	}
}

func TestListVersionsStopsAtFetchCap(t *testing.T) {
	// 1500 matching files: the fetch cap must stop the loop at 1000 with a
	// stderr notice (printed to the test's stderr; asserted via count).
	srv, _ := newFilesServer(t, 1500)
	c := newTestCFClient(srv)
	versions, err := c.ListVersions(context.Background(), "238222", nil, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != cfFilesMaxFetch {
		t.Errorf("versions = %d, want %d", len(versions), cfFilesMaxFetch)
	}
}

func TestListVersionsShortPageTerminates(t *testing.T) {
	// A misbehaving totalCount=0 must not loop forever: a short page ends it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 3 items, pageSize claimed 200 by the client, totalCount reported 0.
		writeFilesPage(w, 3, 200, 0)
	}))
	t.Cleanup(srv.Close)
	c := newTestCFClient(srv)
	versions, err := c.ListVersions(context.Background(), "238222", nil, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("versions = %d, want 3", len(versions))
	}
}

func TestCurseForgeSearchPageSizeCapped(t *testing.T) {
	rec := &queryRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.set(r.URL.Query())
		fmt.Fprint(w, `{"data":[]}`)
	}))
	t.Cleanup(srv.Close)
	c := newTestCFClient(srv)
	if _, err := c.Search(context.Background(), SearchOptions{Query: "sodium", Limit: 200}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	// /mods/search hard-rejects pageSize > 50 with a 500, so the client must
	// clamp to 50 instead of sending a doomed (retried) request.
	if got := rec.get().Get("pageSize"); got != strconv.Itoa(CurseForgeMaxSearchLimit) {
		t.Errorf("pageSize = %q, want %d", got, CurseForgeMaxSearchLimit)
	}
}
