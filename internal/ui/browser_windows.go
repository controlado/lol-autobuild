//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

func OpenBrowser(rawURL string) error {
	cmd := exec.Command("explorer.exe", rawURL)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
