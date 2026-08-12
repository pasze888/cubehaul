//go:build !windows

package netx

import (
	"net/http"
	"net/url"
)

// systemProxyFunc is a no-op on platforms without Windows proxy settings.
// Environment variables are handled by ProxyFunc; macOS/Linux users can set
// HTTP_PROXY/HTTPS_PROXY or CUBEHAUL_PROXY.
func systemProxyFunc() func(*http.Request) (*url.URL, error) {
	return nil
}
