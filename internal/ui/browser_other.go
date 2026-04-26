//go:build !windows && !darwin

package ui

import "errors"

func OpenBrowser(string) error {
	return errors.New("open browser is not supported on this platform")
}
