//go:build windows

package download

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// downloadsDir asks the shell for FOLDERID_Downloads. This honours redirects
// such as OneDrive's Known Folder Move (%OneDrive%\Downloads) and group-policy
// retargeting, which a $USERPROFILE\Downloads guess would miss.
func downloadsDir() (string, error) {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("SHGetKnownFolderPath(FOLDERID_Downloads): %w", err)
	}
	return filepath.Clean(dir), nil
}
