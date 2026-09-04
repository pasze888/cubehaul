package download

import "testing"

func TestParseXdgDownloadDir(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"quoted home", `XDG_DOWNLOAD_DIR="$HOME/Downloads"`, "/home/u/Downloads"},
		{"braced home", `XDG_DOWNLOAD_DIR="${HOME}/Downloads"`, "/home/u/Downloads"},
		{"tilde", `XDG_DOWNLOAD_DIR=~/Downloads`, "/home/u/Downloads"},
		{"localized", `XDG_DOWNLOAD_DIR="$HOME/下载"`, "/home/u/下载"},
		{"absolute", "XDG_DOWNLOAD_DIR=/data/dl", "/data/dl"},
		{"unquoted", "XDG_DOWNLOAD_DIR=$HOME/dl", "/home/u/dl"},
		{"trailing slash", `XDG_DOWNLOAD_DIR="$HOME/Downloads/"`, "/home/u/Downloads/"},
		{"surrounding space", "  XDG_DOWNLOAD_DIR = /data/dl  ", "/data/dl"},
		{"commented out", `#XDG_DOWNLOAD_DIR="$HOME/Downloads"`, ""},
		{"other keys only", `XDG_DOCUMENTS_DIR="$HOME/Documents"`, ""},
		{"empty value", `XDG_DOWNLOAD_DIR=""`, ""},
		{"relative", "XDG_DOWNLOAD_DIR=Downloads", ""},
		{"no equals sign", "XDG_DOWNLOAD_DIR", ""},
		{"first key wins", "XDG_DOWNLOAD_DIR=/a\nXDG_DOWNLOAD_DIR=/b", "/a"},
		{"crlf", "XDG_DOWNLOAD_DIR=/data/dl\r\n", "/data/dl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseXdgDownloadDir(tt.content, "/home/u"); got != tt.want {
				t.Errorf("parseXdgDownloadDir(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Skipf("no system Downloads folder on this host: %v", err)
	}
	if dir == "" {
		t.Fatal("DefaultDir returned an empty path")
	}
	t.Logf("system Downloads dir: %s", dir)
}
