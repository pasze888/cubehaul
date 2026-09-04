package download

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultDir returns the directory used when the caller does not specify one:
// the current user's system Downloads folder.
//
// It returns an error when the folder cannot be determined — no home
// directory, or a failing known-folder lookup on Windows. Callers should then
// require an explicit output dir rather than silently writing somewhere
// surprising.
func DefaultDir() (string, error) {
	dir, err := downloadsDir()
	if err != nil {
		return "", err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("system Downloads folder resolved to an empty path")
	}
	return filepath.Clean(dir), nil
}

// xdgUserDirsFile is the freedesktop definition of the per-user special
// folders; on Linux the Downloads folder is localized and lives in here as
// XDG_DOWNLOAD_DIR (e.g. ~/下載), so guessing $HOME/Downloads is wrong.
const xdgUserDirsFile = ".config/user-dirs.dirs"

// parseXdgDownloadDir extracts XDG_DOWNLOAD_DIR from the contents of a
// user-dirs.dirs file, expanding $HOME, ${HOME} and a leading ~. It returns
// "" when the key is absent, commented out, empty or not absolute.
//
// Matching happens on forward slashes only and no filepath.* call is made, so
// the result is purely lexical and this stays unit-testable on every platform
// even though only the Unix backend consumes it. The caller cleans the path.
//
// It lives in this platform-neutral file so it can be unit-tested anywhere.
func parseXdgDownloadDir(content, home string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "XDG_DOWNLOAD_DIR" {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if val == "" {
			return ""
		}
		switch {
		case strings.HasPrefix(val, "~"):
			val = home + strings.TrimPrefix(val, "~")
		case strings.HasPrefix(val, "${HOME}"):
			val = home + strings.TrimPrefix(val, "${HOME}")
		case strings.HasPrefix(val, "$HOME"):
			val = home + strings.TrimPrefix(val, "$HOME")
		}
		if !strings.HasPrefix(val, "/") {
			return ""
		}
		return val
	}
	return ""
}
