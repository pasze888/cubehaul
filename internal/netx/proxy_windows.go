//go:build windows

package netx

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// systemProxyFunc returns a proxy function based on the Windows system proxy
// settings (Internet Options), or nil when no proxy is enabled. Parsing and
// bypass matching live in netx.go so they are unit-testable on any platform.
func systemProxyFunc() func(*http.Request) (*url.URL, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()

	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return nil
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || strings.TrimSpace(server) == "" {
		return nil
	}
	override, _, _ := k.GetStringValue("ProxyOverride")

	proxies := parseProxyServer(server)
	if len(proxies) == 0 {
		return nil
	}
	noProxy := parseOverride(override)
	debugf("using system proxy %s", server)
	return func(req *http.Request) (*url.URL, error) {
		if bypass(req.URL.Hostname(), noProxy) {
			return nil, nil
		}
		if u, ok := proxies[req.URL.Scheme]; ok {
			return u, nil
		}
		if u, ok := proxies["http"]; ok {
			return u, nil
		}
		return nil, nil
	}
}
