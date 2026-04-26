package ui

import (
	"io"
)

type Options struct {
	App         App
	OpenBrowser BrowserOpener
	Token       string
	Out         io.Writer
}
