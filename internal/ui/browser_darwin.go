//go:build darwin

package ui

import "os/exec"

func OpenBrowser(rawURL string) error {
	return exec.Command("open", rawURL).Start()
}
