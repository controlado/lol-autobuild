package lcu

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDetectChampionIDReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	client.discoverLockfilePaths = func() []string { return nil }
	_, err := client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestParseLockfileValid(t *testing.T) {
	t.Parallel()

	info, err := parseLockfile([]byte("LeagueClientUx:1234:61538:secret:https"))
	if err != nil {
		t.Fatalf("parseLockfile() error = %v", err)
	}

	if info.Port != 61538 || info.Password != "secret" || info.Protocol != "https" {
		t.Fatalf("unexpected lockfile info: %#v", info)
	}
}

func TestParseLockfileInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseLockfile([]byte("invalid"))
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
}

func TestDetectChampionIDFromChampSelect(t *testing.T) {
	t.Parallel()

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:secret"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lol-champ-select/v1/session" {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Authorization") != expectedAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		_, _ = fmt.Fprint(w, `{"localPlayerCellId":3,"myTeam":[{"cellId":2,"championId":1},{"cellId":3,"championId":240}]}`)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	dir := t.TempDir()
	lockfilePath := filepath.Join(dir, "lockfile")
	raw := fmt.Sprintf("LeagueClientUx:1234:%d:secret:http", port)
	if err := os.WriteFile(lockfilePath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	id, err := client.DetectChampionID(context.Background())
	if err != nil {
		t.Fatalf("DetectChampionID() error = %v", err)
	}

	if id != 240 {
		t.Fatalf("expected champion id 240, got %d", id)
	}
}

func TestDetectChampionIDReturnsNotSelected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":0}]}`)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	raw := fmt.Sprintf("LeagueClientUx:1234:%d:secret:http", port)
	if err := os.WriteFile(lockfilePath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	_, err = client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
}

func TestDetectChampionIDReturnsUnavailableWhenNotInChampSelect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	raw := fmt.Sprintf("LeagueClientUx:1234:%d:secret:http", port)
	if err := os.WriteFile(lockfilePath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	_, err = client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestDetectChampionIDReturnsLockfileNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverLockfilePaths = func() []string { return nil }
	_, err := client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrLockfileNotFound) {
		t.Fatalf("expected ErrLockfileNotFound, got %v", err)
	}
}

func TestDetectChampionIDFallsBackWhenAutoCandidateFails(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":67}]}`)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	id, err := client.DetectChampionID(context.Background())
	if err != nil {
		t.Fatalf("DetectChampionID() error = %v", err)
	}

	if id != 67 {
		t.Fatalf("expected champion id 67, got %d", id)
	}
}

func TestDetectChampionIDFallsBackAfterChampionNotSelected(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":0}]}`)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"localPlayerCellId":2,"myTeam":[{"cellId":2,"championId":777}]}`)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	id, err := client.DetectChampionID(context.Background())
	if err != nil {
		t.Fatalf("DetectChampionID() error = %v", err)
	}

	if id != 777 {
		t.Fatalf("expected champion id 777, got %d", id)
	}
}

func TestDetectChampionIDAllCandidatesFailReturnsPriorityError(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":0}]}`)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	_, err := client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
	if !strings.Contains(err.Error(), "last candidate error:") {
		t.Fatalf("expected last candidate context in error, got %v", err)
	}
}

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
