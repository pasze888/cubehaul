//go:build !windows && !darwin

package browser

import "os/exec"

func open(rawURL string) error {
	return exec.Command("xdg-open", rawURL).Run()
}
