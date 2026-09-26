//go:build windows

package browser

import "os/exec"

// open uses the shell URL handler registered for the URL's protocol.
func open(rawURL string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Run()
}
