//go:build !windows

package download

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// downloadsDir resolves the Downloads folder on macOS and Linux.
//
// Order: an explicit XDG_DOWNLOAD_DIR in the environment, then the same key in
// ~/.config/user-dirs.dirs (Linux writes a localized folder name there, e.g.
// ~/下載, so guessing $HOME/Downloads would be wrong), then $HOME/Downloads.
func downloadsDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_DOWNLOAD_DIR")); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory: %w", err)
	}
	if data, err := os.ReadFile(filepath.Join(home, xdgUserDirsFile)); err == nil {
		if dir := parseXdgDownloadDir(string(data), home); dir != "" {
			return dir, nil
		}
	}
	return filepath.Join(home, "Downloads"), nil
}
