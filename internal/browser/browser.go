// Package browser opens URLs in the user's default browser.
package browser

import (
	"errors"
	"fmt"
)

// Open hands rawURL to the platform's URL opener. It waits for the opener
// process, which returns as soon as the browser has been asked to show the page.
func Open(rawURL string) error {
	if rawURL == "" {
		return errors.New("no URL to open")
	}
	if err := open(rawURL); err != nil {
		return fmt.Errorf("open %s: %w", rawURL, err)
	}
	return nil
}
