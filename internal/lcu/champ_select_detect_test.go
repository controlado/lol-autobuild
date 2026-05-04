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
	"sync/atomic"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestDetectSelectionReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestParseLockfileValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "league client", raw: "LeagueClient:1234:61538:secret:https"},
		{name: "league client ux", raw: "LeagueClientUx:1234:61538:secret:https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info, err := parseLockfile([]byte(tt.raw))
			if err != nil {
				t.Fatalf("parseLockfile() error = %v", err)
			}

			if info.Port != 61538 || info.Password != "secret" || info.Protocol != "https" {
				t.Fatalf("unexpected lockfile info: %#v", info)
			}
		})
	}
}

func TestParseLockfileInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseLockfile([]byte("invalid"))
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
}

func TestParseLockfileRejectsRiotClient(t *testing.T) {
	t.Parallel()

	_, err := parseLockfile([]byte("RiotClientUx:1234:61538:secret:https"))
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported process") {
		t.Fatalf("expected unsupported process error, got %v", err)
	}
}

func TestParseLockfileInvalidProtocol(t *testing.T) {
	t.Parallel()

	_, err := parseLockfile([]byte("LeagueClientUx:1234:61538:secret:ftp"))
	if !errors.Is(err, ErrInvalidLockfile) {
		t.Fatalf("expected ErrInvalidLockfile, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected unsupported protocol error, got %v", err)
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 240 {
		t.Fatalf("expected champion id 240, got %d", selection.ChampionID)
	}
	if selection.Position != "mid" {
		t.Fatalf("expected position mid, got %q", selection.Position)
	}
	if selection.QueueID != 420 {
		t.Fatalf("expected queue id 420, got %d", selection.QueueID)
	}
	if !selection.IsAutofilled {
		t.Fatalf("expected autofill flag true")
	}
}

func TestDetectSelectionResolvesChampionNameFromCachedSummary(t *testing.T) {
	t.Parallel()

	var summaryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"assignedPosition":"TOP","isAutofilled":false}]}`)
		case "/lol-game-data/assets/v1/champion-summary.json":
			summaryCalls.Add(1)
			_, _ = fmt.Fprint(w, `[{"id":240,"name":"Kled"},{"id":777,"name":"Yone"}]`)
		case "/lol-game-data/assets/v1/champions/240.json":
			t.Fatalf("unexpected champion detail lookup")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	for range 2 {
		selection, err := client.DetectSelection(context.Background())
		if err != nil {
			t.Fatalf("DetectSelection() error = %v", err)
		}
		if selection.ChampionName != "Kled" {
			t.Fatalf("ChampionName = %q, want Kled", selection.ChampionName)
		}
	}

	if got := summaryCalls.Load(); got != 1 {
		t.Fatalf("summary calls = %d, want 1", got)
	}
}

func TestDetectSelectionFallsBackToChampionDetailWhenSummaryMisses(t *testing.T) {
	t.Parallel()

	var detailCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"assignedPosition":"TOP","isAutofilled":false}]}`)
		case "/lol-game-data/assets/v1/champion-summary.json":
			_, _ = fmt.Fprint(w, `[{"id":777,"name":"Yone"}]`)
		case "/lol-game-data/assets/v1/champions/240.json":
			detailCalls.Add(1)
			_, _ = fmt.Fprint(w, `{"id":240,"name":"Kled"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}
	if selection.ChampionName != "Kled" {
		t.Fatalf("ChampionName = %q, want Kled", selection.ChampionName)
	}
	if got := detailCalls.Load(); got != 1 {
		t.Fatalf("detail calls = %d, want 1", got)
	}
}

func TestDetectSelectionContinuesWhenChampionNameLookupFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"assignedPosition":"TOP","isAutofilled":false}]}`)
		case "/lol-game-data/assets/v1/champion-summary.json":
			http.Error(w, "summary unavailable", http.StatusInternalServerError)
		case "/lol-game-data/assets/v1/champions/240.json":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}
	if selection.ChampionID != 240 || selection.ChampionName != "" {
		t.Fatalf("selection = %#v, want champion 240 without name", selection)
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
}

func TestDetectSelectionReturnsPositionNotAssigned(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":266,"assignedPosition":"UNSELECTED","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, domain.ErrPositionNotAssigned) {
		t.Fatalf("expected domain.ErrPositionNotAssigned, got %v", err)
	}
}

func TestDetectSelectionReturnsPositionUnknown(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":266,"assignedPosition":"DUO_TOP","isAutofilled":false}]}`)
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, port)

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, domain.ErrPositionUnknown) {
		t.Fatalf("expected domain.ErrPositionUnknown, got %v", err)
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrPositionDetectionUnsupportedQueue) {
		t.Fatalf("expected ErrPositionDetectionUnsupportedQueue, got %v", err)
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestDetectSelectionReturnsLockfileNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 67 || selection.Position != "adc" || selection.QueueID != 440 {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestDetectSelectionFallsBackToInferredProcessLockfileWhenProcessPortRefused(t *testing.T) {
	t.Parallel()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":440,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":67,"assignedPosition":"BOTTOM","isAutofilled":false}]}`)
	}))
	defer fallbackServer.Close()

	badPort := mustClosedTCPPort(t)
	processDir := t.TempDir()
	writeLockfile(t, filepath.Join(processDir, "lockfile"), mustServerPort(t, fallbackServer.URL))

	inferredLockfileCandidate, ok := processLockfileCandidate("process:1234", filepath.Join(processDir, "LeagueClientUx.exe"))
	if !ok {
		t.Fatal("expected inferred process lockfile candidate")
	}

	client := NewClient(true, "")
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{
			staticCandidate("process:1234", connectionInfo{Port: badPort, Password: "secret", Protocol: "http"}),
			inferredLockfileCandidate,
		}
	}

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 67 || selection.Position != "adc" || selection.QueueID != 440 {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestDetectSelectionFallsBackAfterPositionNotAssigned(t *testing.T) {
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	selection, err := client.DetectSelection(context.Background())
	if err != nil {
		t.Fatalf("DetectSelection() error = %v", err)
	}

	if selection.ChampionID != 777 || selection.Position != "support" || !selection.IsAutofilled {
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{Port: autoPort, Password: "secret", Protocol: "http"})}
	}

	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
}
