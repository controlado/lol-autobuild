package lcu

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLCUProcessArgsSuccess(t *testing.T) {
	t.Parallel()

	info, err := parseProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port", "61538",
		"--remoting-auth-token=secret-token",
		"--app-protocol", "http",
	})
	if err != nil {
		t.Fatalf("parseProcessArgs() error = %v", err)
	}

	if info.Port != 61538 {
		t.Fatalf("expected port 61538, got %d", info.Port)
	}
	if info.Password != "secret-token" {
		t.Fatalf("expected remoting auth token to be parsed")
	}
	if info.Protocol != "http" {
		t.Fatalf("expected protocol http, got %q", info.Protocol)
	}
}

func TestParseLCUProcessArgsFailsWhenPortMissing(t *testing.T) {
	t.Parallel()

	_, err := parseProcessArgs([]string{
		"LeagueClientUx.exe",
		"--remoting-auth-token=secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "--app-port") {
		t.Fatalf("expected missing --app-port error, got %v", err)
	}
}

func TestParseLCUProcessArgsFailsWhenPortInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port=invalid",
		"--remoting-auth-token=secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "--app-port") {
		t.Fatalf("expected invalid --app-port error, got %v", err)
	}
}

func TestParseLCUProcessArgsFailsWhenTokenMissing(t *testing.T) {
	t.Parallel()

	_, err := parseProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port=61538",
	})
	if err == nil || !strings.Contains(err.Error(), "--remoting-auth-token") {
		t.Fatalf("expected missing --remoting-auth-token error, got %v", err)
	}
}

func TestParseLCUProcessArgsInvalidProtocolFallsBackToDefault(t *testing.T) {
	t.Parallel()

	info, err := parseProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port=61538",
		"--remoting-auth-token=secret-token",
		"--app-protocol=something-else",
	})
	if err != nil {
		t.Fatalf("parseProcessArgs() error = %v", err)
	}

	if info.Protocol != defaultLCUAppProtocol {
		t.Fatalf("expected protocol fallback %q, got %q", defaultLCUAppProtocol, info.Protocol)
	}
}

func TestProcessLockfileCandidateUsesExecutableDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLockfile(t, filepath.Join(dir, "lockfile"), 61538)

	candidate, ok := processLockfileCandidate("process:1234", filepath.Join(dir, "LeagueClientUx.exe"))
	if !ok {
		t.Fatal("expected process lockfile candidate")
	}
	if candidate.label() != "process:1234:lockfile" {
		t.Fatalf("unexpected candidate label: %q", candidate.label())
	}

	info, err := candidate.resolve()
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if info.Port != 61538 || info.Password != "secret" || info.Protocol != "http" {
		t.Fatalf("unexpected lockfile info: %#v", info)
	}
}
