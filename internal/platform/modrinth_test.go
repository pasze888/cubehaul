package platform

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestClampSearchLimit(t *testing.T) {
	tests := []struct {
		name       string
		plat       string
		limit, max int
		want       int
		wantNotice bool
	}{
		{"within limit passes through", "modrinth", 100, ModrinthMaxSearchLimit, 100, false},
		{"zero passes through", "modrinth", 0, ModrinthMaxSearchLimit, 0, false},
		{"modrinth cap", "modrinth", 200, ModrinthMaxSearchLimit, 100, true},
		{"curseforge cap", "curseforge", 200, CurseForgeMaxSearchLimit, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notice := clampSearchLimit(tt.plat, tt.limit, tt.max)
			if got != tt.want {
				t.Errorf("clampSearchLimit(%s, %d) = %d, want %d", tt.plat, tt.limit, got, tt.want)
			}
			if (notice != "") != tt.wantNotice {
				t.Errorf("notice = %q, wantNotice = %v", notice, tt.wantNotice)
			}
			if tt.wantNotice && !strings.Contains(notice, "page with --offset") {
				t.Errorf("notice %q should point at --offset", notice)
			}
		})
	}
}

// Captures the limit query param ModrinthClient.Search sends.
type modrinthQueryRecorder struct {
	mu  sync.Mutex
	val string // captured q.Get("limit")
}

func TestModrinthSearchLimitCapped(t *testing.T) {
	rec := &modrinthQueryRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.val = r.URL.Query().Get("limit")
		rec.mu.Unlock()
		fmt.Fprint(w, `{"hits":[],"total_hits":0}`)
	}))
	t.Cleanup(srv.Close)

	c := &ModrinthClient{base: srv.URL, http: srv.Client(), userAgent: "test"}
	if _, err := c.Search(context.Background(), SearchOptions{Query: "sodium", Limit: 200}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	rec.mu.Lock()
	got := rec.val
	rec.mu.Unlock()
	// Modrinth silently clamps above 100 server-side; the client clamps too
	// (same result, one fewer surprise) and announces it on stderr.
	if got != "100" {
		t.Errorf("limit param = %q, want 100", got)
	}
}
