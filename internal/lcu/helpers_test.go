package lcu

import (
	"fmt"
	"net"
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

func mustClosedTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on test port: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close test listener: %v", err)
	}

	return port
}
