package lcu

import (
	"strings"
	"testing"
)

func TestParseLCUProcessArgsSuccess(t *testing.T) {
	t.Parallel()

	info, err := parseLCUProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port", "61538",
		"--remoting-auth-token=secret-token",
		"--app-protocol", "http",
	})
	if err != nil {
		t.Fatalf("parseLCUProcessArgs() error = %v", err)
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

	_, err := parseLCUProcessArgs([]string{
		"LeagueClientUx.exe",
		"--remoting-auth-token=secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "--app-port") {
		t.Fatalf("expected missing --app-port error, got %v", err)
	}
}

func TestParseLCUProcessArgsFailsWhenPortInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseLCUProcessArgs([]string{
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

	_, err := parseLCUProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port=61538",
	})
	if err == nil || !strings.Contains(err.Error(), "--remoting-auth-token") {
		t.Fatalf("expected missing --remoting-auth-token error, got %v", err)
	}
}

func TestParseLCUProcessArgsInvalidProtocolFallsBackToDefault(t *testing.T) {
	t.Parallel()

	info, err := parseLCUProcessArgs([]string{
		"LeagueClientUx.exe",
		"--app-port=61538",
		"--remoting-auth-token=secret-token",
		"--app-protocol=something-else",
	})
	if err != nil {
		t.Fatalf("parseLCUProcessArgs() error = %v", err)
	}

	if info.Protocol != defaultLCUAppProtocol {
		t.Fatalf("expected protocol fallback %q, got %q", defaultLCUAppProtocol, info.Protocol)
	}
}
