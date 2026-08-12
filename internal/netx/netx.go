// Package netx builds HTTP clients that honor the system proxy.
package netx

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// NewClient returns an HTTP client whose transport routes requests through
// the system proxy when one is configured. A timeout of 0 disables the
// overall request timeout (useful for large downloads).
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: ProxyFunc(),
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// ProxyFunc resolves the proxy for a request, in order:
//  1. CUBEHAUL_PROXY — explicit override, e.g. "http://127.0.0.1:7897" or "socks5://127.0.0.1:7890"
//  2. HTTP_PROXY / HTTPS_PROXY / NO_PROXY environment variables (standard Go behavior)
//  3. Windows system proxy (Internet Options: ProxyEnable/ProxyServer/ProxyOverride)
//  4. direct connection
func ProxyFunc() func(*http.Request) (*url.URL, error) {
	if v := os.Getenv("CUBEHAUL_PROXY"); v != "" {
		u, err := parseProxyURL(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cubehaul: invalid CUBEHAUL_PROXY %q: %v (ignored)\n", v, err)
			return http.ProxyFromEnvironment
		}
		debugf("using proxy %s (CUBEHAUL_PROXY)", u)
		return func(*http.Request) (*url.URL, error) { return u, nil }
	}
	if hasEnvProxy() {
		return http.ProxyFromEnvironment
	}
	if f := systemProxyFunc(); f != nil {
		return f
	}
	return func(*http.Request) (*url.URL, error) { return nil, nil }
}

func hasEnvProxy() bool {
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

func parseProxyURL(v string) (*url.URL, error) {
	if !strings.Contains(v, "://") {
		v = "http://" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	return u, nil
}

func debugf(format string, args ...any) {
	if os.Getenv("CUBEHAUL_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "cubehaul: "+format+"\n", args...)
	}
}

// parseProxyServer parses a Windows ProxyServer registry value:
//
//	"127.0.0.1:7897"                   -> one HTTP proxy for all schemes
//	"http=host:8080;https=host:8081"   -> per-scheme proxies
//	"socks=127.0.0.1:7890"             -> SOCKS5 proxy for all schemes
func parseProxyServer(s string) map[string]*url.URL {
	out := map[string]*url.URL{}
	if !strings.Contains(s, "=") {
		if u := proxyURL("http", s); u != nil {
			out["http"] = u
			out["https"] = u
		}
		return out
	}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		scheme, addr, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(scheme) {
		case "http", "https":
			if u := proxyURL("http", addr); u != nil {
				out[strings.ToLower(scheme)] = u
			}
		case "socks":
			if u := proxyURL("socks5", addr); u != nil {
				out["http"] = u
				out["https"] = u
			}
		}
	}
	return out
}

func proxyURL(scheme, addr string) *url.URL {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	u, err := url.Parse(scheme + "://" + addr)
	if err != nil {
		return nil
	}
	return u
}

// parseOverride converts a Windows ProxyOverride registry value into a
// bypass list.
func parseOverride(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "<-loopback>" { // <-loopback>: do NOT bypass loopback
			continue
		}
		out = append(out, strings.ToLower(p))
	}
	return out
}

// bypass reports whether host should connect directly instead of via the
// proxy. Supports the common ProxyOverride forms: "*", "<local>", "127.*",
// "*.example.com", "example.com", and exact IPs.
func bypass(host string, entries []string) bool {
	if len(entries) == 0 {
		return false
	}
	host = strings.ToLower(host)
	for _, e := range entries {
		switch {
		case e == "*":
			return true
		case e == "<local>":
			if host == "localhost" {
				return true
			}
			if ip := net.ParseIP(host); ip != nil &&
				(ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
				return true
			}
		case strings.HasSuffix(e, ".*"):
			if strings.HasPrefix(host, strings.TrimSuffix(e, "*")) {
				return true
			}
		default:
			e = strings.TrimPrefix(e, "*.")
			if host == e || strings.HasSuffix(host, "."+e) {
				return true
			}
		}
	}
	return false
}
