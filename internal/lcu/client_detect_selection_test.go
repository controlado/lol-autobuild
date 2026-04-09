package lcu

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectSelectionReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
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

func TestDetectSelectionFromChampSelect(t *testing.T) {
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

		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":2,"championId":1,"assignedPosition":"TOP","isAutofilled":false},{"cellId":3,"championId":240,"assignedPosition":"MIDDLE","isAutofilled":true}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 240 {
		t.Fatalf("expected champion id 240, got %d", selection.ChampionID)
	}
	if selection.Role != "mid" {
		t.Fatalf("expected role mid, got %q", selection.Role)
	}
	if selection.QueueID != 420 {
		t.Fatalf("expected queue id 420, got %d", selection.QueueID)
	}
	if !selection.IsAutofilled {
		t.Fatalf("expected autofill flag true")
	}
}

func TestDetectSelectionReturnsChampionNotSelected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":0,"assignedPosition":"TOP","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
}

func TestDetectSelectionReturnsRoleNotAssigned(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":266,"assignedPosition":"UNSELECTED","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrRoleNotAssigned) {
		t.Fatalf("expected ErrRoleNotAssigned, got %v", err)
	}
}

func TestDetectSelectionReturnsRoleUnknown(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":266,"assignedPosition":"DUO_TOP","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrRoleUnknown) {
		t.Fatalf("expected ErrRoleUnknown, got %v", err)
	}
}

func TestDetectSelectionReturnsUnsupportedQueue(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":490,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":55,"assignedPosition":"MIDDLE","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrRoleDetectionUnsupportedQueue) {
		t.Fatalf("expected ErrRoleDetectionUnsupportedQueue, got %v", err)
	}
}

func TestDetectSelectionReturnsUnavailableWhenNotInChampSelect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestDetectSelectionReturnsLockfileNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrLockfileNotFound) {
		t.Fatalf("expected ErrLockfileNotFound, got %v", err)
	}
}

func TestDetectSelectionFallsBackWhenProcessCandidateFails(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":440,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":67,"assignedPosition":"BOTTOM","isAutofilled":false}]}`)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	fallbackPath := filepath.Join(t.TempDir(), "fallback.lockfile")
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate {
		return []clientConnectionCandidate{staticConnectionCandidate("process:1234", lockfileInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 67 || selection.Role != "adc" || selection.QueueID != 440 {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestDetectSelectionFallsBackAfterRoleNotAssigned(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":99,"assignedPosition":"UNSELECTED","isAutofilled":false}]}`)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":2,"myTeam":[{"cellId":2,"championId":777,"assignedPosition":"UTILITY","isAutofilled":true}]}`)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	fallbackPath := filepath.Join(t.TempDir(), "fallback.lockfile")
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate {
		return []clientConnectionCandidate{staticConnectionCandidate("process:1234", lockfileInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 777 || selection.Role != "support" || !selection.IsAutofilled {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestDetectSelectionAllCandidatesFailReturnsPriorityError(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":490,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":157,"assignedPosition":"MIDDLE","isAutofilled":false}]}`)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":0,"assignedPosition":"TOP","isAutofilled":false}]}`)
	}))
	defer fallbackServer.Close()

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPort := mustServerPort(t, fallbackServer.URL)

	fallbackPath := filepath.Join(t.TempDir(), "fallback.lockfile")
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate {
		return []clientConnectionCandidate{staticConnectionCandidate("process:1234", lockfileInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
	if !strings.Contains(err.Error(), "last candidate error:") {
		t.Fatalf("expected last candidate context in error, got %v", err)
	}
}
