package lcu

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func mustServerPort(t *testing.T, rawURL string) int {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	return port
}

func writeLockfile(t *testing.T, path string, port int) {
	t.Helper()

	raw := fmt.Sprintf("LeagueClientUx:1234:%d:secret:http", port)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
}
