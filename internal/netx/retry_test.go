package netx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryStatus(t *testing.T) {
	retryable := []int{429, 500, 502, 503, 504}
	for _, code := range retryable {
		if !retryStatus(code) {
			t.Errorf("retryStatus(%d) = false, want true", code)
		}
	}
	final := []int{200, 201, 301, 400, 401, 403, 404, 410, 422, 501, 505}
	for _, code := range final {
		if retryStatus(code) {
			t.Errorf("retryStatus(%d) = true, want false", code)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	tests := []struct {
		name    string
		in      string
		wantMin time.Duration // 0 means "expect exactly 0"
		wantMax time.Duration
	}{
		{"seconds", "2", 2 * time.Second, 2 * time.Second},
		{"zero", "0", 0, 0},
		{"negative", "-3", 0, 0},
		{"empty", "", 0, 0},
		{"garbage", "soon", 0, 0},
		{"http date future", future, 30 * time.Second, time.Minute},
		{"http date past", past, 0, 0},
		{"surrounding space", " 5 ", 5 * time.Second, 5 * time.Second},
	}
	for _, tt := range tests {
		got := parseRetryAfter(tt.in)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("parseRetryAfter(%q) = %v, want in [%v, %v]", tt.in, got, tt.wantMin, tt.wantMax)
		}
	}
}

func TestNextWait(t *testing.T) {
	base, maxWait := 750*time.Millisecond, 8*time.Second
	// Retry-After wins over the backoff, capped at retryAfterCap.
	if got := nextWait("2", base, maxWait, 1); got != 2*time.Second {
		t.Errorf("nextWait(\"2\") = %v, want 2s", got)
	}
	if got := nextWait("3600", base, maxWait, 1); got != retryAfterCap {
		t.Errorf("nextWait(\"3600\") = %v, want capped at %v", got, retryAfterCap)
	}
	// Absent or invalid Retry-After falls back to the exponential backoff.
	for _, ra := range []string{"", "garbage"} {
		got := nextWait(ra, base, maxWait, 2)
		if got <= 0 || got > maxWait {
			t.Errorf("nextWait(%q, attempt 2) = %v, want in (0, %v]", ra, got, maxWait)
		}
	}
}

func TestBackoffWaitBounds(t *testing.T) {
	base, maxWait := 750*time.Millisecond, 8*time.Second
	for attempt := 1; attempt <= 8; attempt++ {
		d := base
		for i := 1; i < attempt && d < maxWait; i++ {
			d *= 2
		}
		d = min(d, maxWait)
		got := backoffWait(base, maxWait, attempt)
		if got < d/2 || got > d {
			t.Errorf("backoffWait(attempt %d) = %v, want in [%v, %v]", attempt, got, d/2, d)
		}
	}
}

// flakyServer answers status n times, then 200 with a JSON body. The client
// always comes from srv.Client(): a loopback transport with no proxy, so the
// tests never depend on the host's proxy environment.
func flakyServer(t *testing.T, status, times int, retryAfter string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= int32(times) {
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(status)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func doFast(ctx context.Context, srv *httptest.Server, attempts int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		return nil, err
	}
	return DoWithRetry(ctx, srv.Client(), req, RetryOptions{
		Attempts: attempts,
		Base:     time.Millisecond,
		Max:      2 * time.Millisecond,
	})
}

func TestDoWithRetryRetriesUntilSuccess(t *testing.T) {
	srv, calls := flakyServer(t, http.StatusTooManyRequests, 2, "0")
	resp, err := doFast(context.Background(), srv, 4)
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok"`) {
		t.Errorf("body = %q, want the success payload", body)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("calls = %d, want 3 (2x429 then 200)", got)
	}
}

func TestDoWithRetryHonorsRetryAfter(t *testing.T) {
	// Retry-After: 0 must still fall back to the backoff, never stall.
	srv, calls := flakyServer(t, http.StatusServiceUnavailable, 1, "0")
	resp, err := doFast(context.Background(), srv, 4)
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("calls = %d, want 2 (503 then 200)", got)
	}
}

func TestDoWithRetryFinalStatusReturned(t *testing.T) {
	// Exhausted attempts on a retryable status hand the last response back;
	// surfacing it is the caller's job.
	srv, calls := flakyServer(t, http.StatusTooManyRequests, 100, "")
	resp, err := doFast(context.Background(), srv, 3)
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("calls = %d, want 3 (attempts exhausted)", got)
	}
}

func TestDoWithRetryNoRetryOn404(t *testing.T) {
	srv, calls := flakyServer(t, http.StatusNotFound, 100, "")
	resp, err := doFast(context.Background(), srv, 4)
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	defer resp.Body.Close()
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("calls = %d, want 1 (404 is final)", got)
	}
}

func TestDoWithRetryTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	client := srv.Client()
	srv.Close() // nothing listens anymore: every attempt fails to connect

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DoWithRetry(context.Background(), client, req, RetryOptions{
		Attempts: 2,
		Base:     time.Millisecond,
		Max:      2 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("DoWithRetry on a dead server should fail")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should be the transport failure, got %v", err)
	}
}

func TestDoWithRetryContextCanceled(t *testing.T) {
	// Cancellation must interrupt the wait between attempts: the server sees
	// exactly one request, then DoWithRetry returns context.Canceled.
	var calls int32
	first := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		select {
		case first <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := DoWithRetry(ctx, srv.Client(), req, RetryOptions{
			Attempts: 4,
			Base:     50 * time.Millisecond,
			Max:      50 * time.Millisecond,
		})
		if resp != nil {
			resp.Body.Close()
		}
		done <- result{resp, err}
	}()

	<-first // the first 429 has been sent; DoWithRetry is entering its wait
	cancel()
	select {
	case r := <-done:
		if r.resp != nil {
			t.Error("canceled ctx should return no response")
		}
		if !errors.Is(r.err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DoWithRetry did not return after cancellation")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 (wait interrupted before the second attempt)", got)
	}
}
