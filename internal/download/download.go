// Package download fetches files over HTTP with progress reporting.
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cubehaul/internal/netx"
	"cubehaul/internal/output"
)

// Options for a single download.
type Options struct {
	URL       string
	Filename  string
	Dir       string
	Size      int64  // expected size in bytes; 0 = unknown
	UserAgent string // sent as the User-Agent header; callers pass the configured one
}

// client is created lazily in Run: it shares the system-proxy-aware
// transport and has no overall timeout, as large files may take a while
// over slow connections.
var (
	clientOnce sync.Once
	client     *http.Client
)

func httpClient() *http.Client {
	clientOnce.Do(func() { client = netx.NewClient(0) })
	return client
}

// Result describes a completed download.
type Result struct {
	Path string
	Size int64
}

// Run downloads o.URL into o.Dir, saving as o.Filename (sanitized). The file
// is written to a .part temp file first, and the size is verified against
// o.Size when known.
func Run(ctx context.Context, o Options) (res *Result, err error) {
	if o.Dir == "" {
		o.Dir = "."
	}
	if err := os.MkdirAll(o.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir %s: %w", o.Dir, err)
	}
	name := sanitizeFilename(o.Filename)
	if name == "" {
		name = "download"
	}
	dest := filepath.Join(o.Dir, name)
	tmp := dest + ".part"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
	if err != nil {
		return nil, err
	}
	if o.UserAgent != "" {
		req.Header.Set("User-Agent", o.UserAgent)
	}
	resp, err := netx.DoWithRetry(ctx, httpClient(), req, netx.RetryOptions{})
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", o.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d %s", o.URL, resp.StatusCode, resp.Status)
	}

	out, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", tmp, err)
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(tmp)
		}
	}()

	total := o.Size
	if total == 0 {
		if n, perr := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); perr == nil {
			total = n
		}
	}
	progress := newProgress(os.Stderr)
	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				err = werr
				return nil, fmt.Errorf("write %s: %w", tmp, werr)
			}
			written += int64(n)
			progress.report(written, total)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			err = rerr
			return nil, fmt.Errorf("read %s: %w", o.URL, rerr)
		}
	}
	progress.done(written, total)
	if err := out.Close(); err != nil {
		return nil, fmt.Errorf("close %s: %w", tmp, err)
	}
	if o.Size > 0 && written != o.Size {
		os.Remove(tmp)
		return nil, fmt.Errorf("size mismatch for %s: expected %d bytes, got %d", name, o.Size, written)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(dest) // Windows cannot rename over an existing file
		if err2 := os.Rename(tmp, dest); err2 != nil {
			return nil, fmt.Errorf("move %s: %w", tmp, err2)
		}
	}
	return &Result{Path: dest, Size: written}, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

type progress struct {
	out  *os.File
	term bool
	last time.Time
}

func newProgress(out *os.File) *progress {
	fi, err := out.Stat()
	return &progress{out: out, term: err == nil && fi.Mode()&os.ModeCharDevice != 0}
}

func (p *progress) report(written, total int64) {
	if !p.term {
		return
	}
	now := time.Now()
	if now.Sub(p.last) < 200*time.Millisecond {
		return
	}
	p.last = now
	if total > 0 {
		pct := float64(written) / float64(total) * 100
		fmt.Fprintf(p.out, "\r  %s / %s (%.1f%%)", output.HumanSize(written), output.HumanSize(total), pct)
	} else {
		fmt.Fprintf(p.out, "\r  %s", output.HumanSize(written))
	}
}

func (p *progress) done(written, total int64) {
	if !p.term {
		return
	}
	if total > 0 {
		fmt.Fprintf(p.out, "\r  %s / %s (100%%)\n", output.HumanSize(written), output.HumanSize(total))
	} else {
		fmt.Fprintf(p.out, "\r  %s\n", output.HumanSize(written))
	}
}
