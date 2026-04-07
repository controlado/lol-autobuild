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
	"testing"
)

func TestDetectChampionIDReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
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
	_, err = client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestDetectChampionIDReturnsLockfileNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	_, err := client.DetectChampionID(context.Background())
	if !errors.Is(err, ErrLockfileNotFound) {
		t.Fatalf("expected ErrLockfileNotFound, got %v", err)
	}
}
