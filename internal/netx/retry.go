package netx

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultRetryAttempts is the total number of attempts DoWithRetry makes
// (first try plus three retries) when RetryOptions.Attempts is unset. It is
// sized to survive a burst of rate limiting without turning a hard failure
// into minutes of silence.
const DefaultRetryAttempts = 4

// retryAfterCap bounds how long a server-supplied Retry-After may stall the
// CLI. A server asking for longer still gets its remaining attempts, but the
// overall wait stays bounded; if it keeps answering 429 the final response
// surfaces the problem to the caller.
const retryAfterCap = 30 * time.Second

// retryStatus reports whether an HTTP status is transient and worth another
// attempt: rate limiting and the classic "bad moment on the server" codes.
// Everything else — success, informative 4xx, exotic 5xx — is final.
func retryStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429: rate limited
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}

// RetryOptions tunes DoWithRetry. The zero value is the default policy:
// DefaultRetryAttempts attempts with exponential backoff from 750ms capped
// at 8s.
type RetryOptions struct {
	// Attempts is the total number of tries including the first; <= 0
	// selects DefaultRetryAttempts.
	Attempts int
	// Base is the first backoff delay; <= 0 means 750ms.
	Base time.Duration
	// Max is the backoff ceiling; <= 0 means 8s. Retry-After is capped
	// separately, at retryAfterCap.
	Max time.Duration
}

// DoWithRetry sends req via client, retrying transient failures: transport
// errors and HTTP 429/500/502/503/504. req must carry no body — every
// cubehaul call is a GET — so resending the same request is safe.
//
// The wait doubles per failed attempt (starting at Base) with jitter over its
// upper half, and a Retry-After header on the failed attempt overrides the
// backoff, capped at retryAfterCap. Intermediate response bodies are closed.
// Each retry is announced on stderr so a slow stall is never silent, and ctx
// cancellation interrupts the wait and aborts the loop.
//
// The last attempt's outcome is returned as-is: a final 429 comes back as a
// normal response for the caller's status handling, a final transport error
// as a Go error.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, opts RetryOptions) (*http.Response, error) {
	attempts := opts.Attempts
	if attempts <= 0 {
		attempts = DefaultRetryAttempts
	}
	base := opts.Base
	if base <= 0 {
		base = 750 * time.Millisecond
	}
	maxWait := opts.Max
	if maxWait <= 0 {
		maxWait = 8 * time.Second
	}

	for attempt := 1; ; attempt++ {
		resp, err := client.Do(req)
		if err == nil && !retryStatus(resp.StatusCode) {
			return resp, nil
		}
		if attempt >= attempts {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		var reason, retryAfter string
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			reason = err.Error()
		} else {
			reason = resp.Status
			retryAfter = resp.Header.Get("Retry-After")
			resp.Body.Close()
		}
		wait := nextWait(retryAfter, base, maxWait, attempt)
		fmt.Fprintf(os.Stderr, "cubehaul: %s %s: %s, retrying in %s (attempt %d/%d)\n",
			req.Method, req.URL.Host+req.URL.Path, reason, wait.Round(time.Millisecond), attempt+1, attempts)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// nextWait picks the wait before the next attempt: the failed attempt's
// Retry-After when present (capped at retryAfterCap), otherwise the jittered
// exponential backoff.
func nextWait(retryAfter string, base, maxWait time.Duration, attempt int) time.Duration {
	if d := parseRetryAfter(retryAfter); d > 0 {
		return min(d, retryAfterCap)
	}
	return backoffWait(base, maxWait, attempt)
}

// parseRetryAfter reads a Retry-After header value, which RFC 9110 allows to
// be either delay-seconds or an HTTP date. Invalid, negative or past values
// yield 0, meaning "use the backoff instead".
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoffWait doubles Base once per prior attempt up to maxWait, then jitters
// over the upper half ([d/2, d]) so concurrent clients do not retry in
// lockstep.
func backoffWait(base, maxWait time.Duration, attempt int) time.Duration {
	d := base
	for i := 1; i < attempt && d < maxWait; i++ {
		d *= 2
	}
	if d > maxWait {
		d = maxWait
	}
	if d <= 0 {
		return 0
	}
	span := d - d/2
	return d/2 + time.Duration(rand.Int64N(int64(span)+1))
}
