package lcu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/controlado/lol-autobuild/internal/ports"
)

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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
			client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged, got %v", err)
	}
}

func TestApplySummonerSpellsFallsBackWhenProcessCandidateFails(t *testing.T) {
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

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPath := filepath.Join(t.TempDir(), "fallback.lockfile")
	writeLockfile(t, fallbackPath, mustServerPort(t, fallbackServer.URL))

	client := NewClient(true, fallbackPath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{
			Port:     autoPort,
			Password: "secret",
			Protocol: "http",
		})}
	}

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
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
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

	autoPort := mustServerPort(t, autoServer.URL)
	fallbackPath := filepath.Join(t.TempDir(), "fallback.lockfile")
	writeLockfile(t, fallbackPath, mustServerPort(t, fallbackServer.URL))

	client := NewClient(true, fallbackPath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{
			Port:     autoPort,
			Password: "secret",
			Protocol: "http",
		})}
	}

	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged priority, got %v", err)
	}
}

func TestApplySummonerSpellsFailsWhenChampionIsNotSelected(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":0,"spell1Id":4,"spell2Id":14}]}`)
	}))
	defer srv.Close()

	client := NewClient(true, "")
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{staticCandidate("process:1234", connectionInfo{
			Port:     mustServerPort(t, srv.URL),
			Password: "secret",
			Protocol: "http",
		})}
	}

	err := client.ApplySummonerSpells(context.Background(), ports.ApplySummonerSpellsRequest{
		ChampionID: 240,
		SpellIDs:   []int{4, 14},
	})
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected priority, got %v", err)
	}
}
