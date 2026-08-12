package netx

import "testing"

func TestParseProxyServer(t *testing.T) {
	tests := []struct {
		in      string
		wantLen int
		wantURL string
	}{
		{"127.0.0.1:7897", 2, "http://127.0.0.1:7897"},
		{"http=host:8080;https=host:8081", 2, "http://host:8080"},
		{"socks=127.0.0.1:7890", 2, "socks5://127.0.0.1:7890"},
		{"  ", 0, ""},
	}
	for _, tt := range tests {
		got := parseProxyServer(tt.in)
		if len(got) != tt.wantLen {
			t.Errorf("parseProxyServer(%q) = %d entries, want %d", tt.in, len(got), tt.wantLen)
			continue
		}
		if tt.wantURL != "" && got["http"] != nil && got["http"].String() != tt.wantURL {
			t.Errorf("parseProxyServer(%q) http = %s, want %s", tt.in, got["http"], tt.wantURL)
		}
	}
	// per-scheme: https must point at the https entry
	got := parseProxyServer("http=a:1;https=b:2")
	if got["http"].String() != "http://a:1" || got["https"].String() != "http://b:2" {
		t.Errorf("per-scheme parse wrong: %v", got)
	}
}

func TestParseOverride(t *testing.T) {
	got := parseOverride("localhost;127.*;10.*;<local>;<-loopback>;")
	if len(got) != 4 {
		t.Fatalf("parseOverride = %v, want 4 entries (drops empty and <-loopback>)", got)
	}
	if got[0] != "localhost" || got[1] != "127.*" || got[2] != "10.*" || got[3] != "<local>" {
		t.Errorf("parseOverride = %v", got)
	}
	if parseOverride("") != nil {
		t.Error("empty override should be nil")
	}
}

func TestBypass(t *testing.T) {
	entries := parseOverride("localhost;127.*;10.*;192.168.*;*.example.com;intranet;<local>")
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"10.0.0.5", true},        // 10.* prefix
		{"192.168.1.20", true},    // 192.168.* prefix
		{"cdn.example.com", true}, // *.example.com suffix
		{"example.com", true},
		{"api.modrinth.com", false},
		{"cdn.modrinth.com", false},
		{"example.org", false},
	}
	for _, c := range cases {
		if got := bypass(c.host, entries); got != c.want {
			t.Errorf("bypass(%q) = %v, want %v", c.host, got, c.want)
		}
	}
	if bypass("127.0.0.1", nil) {
		t.Error("empty bypass list must not bypass")
	}
}
