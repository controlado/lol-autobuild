package lcu

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

	"github.com/controlado/lol-autobuild/internal/ports"
)

func TestDetectSelectionReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
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
	client.discoverLockfilePaths = func() []string { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestDetectSelectionReturnsLockfileNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverLockfilePaths = func() []string { return nil }
	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrLockfileNotFound) {
		t.Fatalf("expected ErrLockfileNotFound, got %v", err)
	}
}

func TestDetectSelectionFallsBackWhenAutoCandidateFails(t *testing.T) {
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

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

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

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

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

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, autoPort)
	writeLockfile(t, fallbackPath, fallbackPort)

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	_, err := client.DetectSelection(context.Background())
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected, got %v", err)
	}
	if !strings.Contains(err.Error(), "last candidate error:") {
		t.Fatalf("expected last candidate context in error, got %v", err)
	}
}

func TestApplySummonerSpellsReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestApplySummonerSpellsDryRunSkipsIO(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverLockfilePaths = func() []string { return nil }

	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("expected nil error in dry-run, got %v", err)
	}
}

func TestApplySummonerSpellsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  ports.ApplySummonerSpellsRequest
	}{
		{
			name: "champion must be > 0",
			req: ports.ApplySummonerSpellsRequest{
				ChampionID: 0,
				SpellIDs:   []int{4, 14},
			},
		},
		{
			name: "requires exactly two spells",
			req: ports.ApplySummonerSpellsRequest{
				ChampionID: 240,
				SpellIDs:   []int{4},
			},
		},
		{
			name: "spell ids must be > 0",
			req: ports.ApplySummonerSpellsRequest{
				ChampionID: 240,
				SpellIDs:   []int{4, 0},
			},
		},
		{
			name: "spell ids must be distinct",
			req: ports.ApplySummonerSpellsRequest{
				ChampionID: 240,
				SpellIDs:   []int{4, 4},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
			client.discoverLockfilePaths = func() []string { return nil }

			err := client.ApplySummonerSpells(context.Background(), tt.req)
			if !errors.Is(err, ErrInvalidSummonerSpellsRequest) {
				t.Fatalf("expected ErrInvalidSummonerSpellsRequest, got %v", err)
			}
		})
	}
}

func TestApplySummonerSpellsSuccess(t *testing.T) {
	t.Parallel()

	type patchPayload struct {
		Spell1ID int `json:"spell1Id"`
		Spell2ID int `json:"spell2Id"`
	}

	var gotPatch patchPayload
	patchCalled := false
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:secret"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET session, got %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-champ-select/v1/session/my-selection":
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH my-selection, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != expectedAuth {
				t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("expected application/json content-type, got %q", r.Header.Get("Content-Type"))
			}
			if err := json.NewDecoder(r.Body).Decode(&gotPatch); err != nil {
				t.Fatalf("decode patch body: %v", err)
			}
			patchCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if err != nil {
		t.Fatalf("ApplySummonerSpells() error = %v", err)
	}
	if !patchCalled {
		t.Fatalf("expected patch endpoint to be called")
	}
	if gotPatch.Spell1ID != 4 || gotPatch.Spell2ID != 14 {
		t.Fatalf("unexpected patch payload: %#v", gotPatch)
	}
}

func TestApplySummonerSpellsPreservesFlashSlot(t *testing.T) {
	t.Parallel()

	type patchPayload struct {
		Spell1ID int `json:"spell1Id"`
		Spell2ID int `json:"spell2Id"`
	}

	var gotPatch patchPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"spell1Id":14,"spell2Id":4}]}`)
		case "/lol-champ-select/v1/session/my-selection":
			if err := json.NewDecoder(r.Body).Decode(&gotPatch); err != nil {
				t.Fatalf("decode patch body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 7},
	})
	if err != nil {
		t.Fatalf("ApplySummonerSpells() error = %v", err)
	}
	if gotPatch.Spell1ID != 7 || gotPatch.Spell2ID != 4 {
		t.Fatalf("expected flash to remain in spell2 slot, got %#v", gotPatch)
	}
}

func TestApplySummonerSpellsFailsWhenChampionChanges(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":157,"spell1Id":4,"spell2Id":14}]}`)
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged, got %v", err)
	}
}

func TestApplySummonerSpellsFallsBackAcrossLockfiles(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":157,"spell1Id":4,"spell2Id":14}]}`)
	}))
	defer autoServer.Close()

	fallbackPatched := false
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":2,"myTeam":[{"cellId":2,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-champ-select/v1/session/my-selection":
			fallbackPatched = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fallbackServer.Close()

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, mustServerPort(t, autoServer.URL))
	writeLockfile(t, fallbackPath, mustServerPort(t, fallbackServer.URL))

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if err != nil {
		t.Fatalf("ApplySummonerSpells() error = %v", err)
	}
	if !fallbackPatched {
		t.Fatalf("expected fallback lockfile candidate to patch my-selection")
	}
}

func TestApplySummonerSpellsNotFoundMapsToChampSelectUnavailable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":3,"myTeam":[{"cellId":3,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-champ-select/v1/session/my-selection":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverLockfilePaths = func() []string { return nil }
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampSelectUnavailable) {
		t.Fatalf("expected ErrChampSelectUnavailable, got %v", err)
	}
}

func TestApplySummonerSpellsAllCandidatesFailRespectsPriority(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":157,"spell1Id":4,"spell2Id":14}]}`)
	}))
	defer autoServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":2,"myTeam":[{"cellId":2,"championId":0,"spell1Id":4,"spell2Id":14}]}`)
	}))
	defer fallbackServer.Close()

	dir := t.TempDir()
	autoPath := filepath.Join(dir, "auto.lockfile")
	fallbackPath := filepath.Join(dir, "fallback.lockfile")
	writeLockfile(t, autoPath, mustServerPort(t, autoServer.URL))
	writeLockfile(t, fallbackPath, mustServerPort(t, fallbackServer.URL))

	client := NewClient(true, fallbackPath)
	client.discoverLockfilePaths = func() []string { return []string{autoPath} }

	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged priority, got %v", err)
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
