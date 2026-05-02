package lcu

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeLCUProcessName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{name: "league client", raw: "LeagueClient", want: "leagueclient", wantOK: true},
		{name: "league client exe", raw: "LeagueClient.exe", want: "leagueclient", wantOK: true},
		{name: "league client ux", raw: "LeagueClientUx", want: "leagueclientux", wantOK: true},
		{name: "league client ux exe", raw: "LeagueClientUx.exe", want: "leagueclientux", wantOK: true},
		{name: "windows path", raw: `C:\Riot Games\League of Legends\LeagueClientUx.exe`, want: "leagueclientux", wantOK: true},
		{name: "unix path", raw: "/opt/riot/LeagueClient", want: "leagueclient", wantOK: true},
		{name: "quoted mixed case path", raw: `"C:\Riot Games\League of Legends\LEAGUECLIENTUX.EXE"`, want: "leagueclientux", wantOK: true},
		{name: "quoted with spaces", raw: `  "LeagueClientUx.exe"  `, want: "leagueclientux", wantOK: true},
		{name: "riot client", raw: "RiotClient", wantOK: false},
		{name: "riot client ux", raw: "RiotClientUx.exe", wantOK: false},
		{name: "riot client services", raw: "RiotClientServices.exe", wantOK: false},
		{name: "empty", raw: "", wantOK: false},
		{name: "similar league process", raw: "LeagueClientUxRender.exe", wantOK: false},
		{name: "contains league client", raw: "not-LeagueClient.exe", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := normalizeLCUProcessName(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("normalizeLCUProcessName(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("normalizeLCUProcessName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

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

func TestParseLCUProcessArgsAcceptsLeagueClient(t *testing.T) {
	t.Parallel()

	info, err := parseProcessArgs([]string{
		"LeagueClient.exe",
		"--app-port", "61538",
		"--remoting-auth-token=secret-token",
	})
	if err != nil {
		t.Fatalf("parseProcessArgs() error = %v", err)
	}

	if info.Port != 61538 || info.Password != "secret-token" {
		t.Fatalf("unexpected connection info: %#v", info)
	}
}

func TestParseLCUProcessArgsRejectsRiotClient(t *testing.T) {
	t.Parallel()

	_, err := parseProcessArgs([]string{
		"RiotClientUx.exe",
		"--app-port", "61538",
		"--remoting-auth-token=secret-token",
	})
	if !errors.Is(err, errUnsupportedProcess) {
		t.Fatalf("expected errUnsupportedProcess, got %v", err)
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

func TestProcessLockfileCandidateTrimsQuotedExecutablePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLockfile(t, filepath.Join(dir, "lockfile"), 61538)

	candidate, ok := processLockfileCandidate("process:1234", `"`+filepath.Join(dir, "LeagueClientUx.exe")+`"`)
	if !ok {
		t.Fatal("expected process lockfile candidate")
	}

	info, err := candidate.resolve()
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if info.Port != 61538 {
		t.Fatalf("expected port 61538, got %d", info.Port)
	}
}

func TestProcessLockfileCandidateRejectsRiotClient(t *testing.T) {
	t.Parallel()

	_, ok := processLockfileCandidate("process:1234", filepath.Join(t.TempDir(), "RiotClientUx.exe"))
	if ok {
		t.Fatal("expected Riot Client executable to be rejected")
	}
}
