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

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestApplyItemSetReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006}},
		},
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestApplyItemSetDryRunSkipsIO(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006}},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("expected nil error in dry-run, got %v", err)
	}
}

func TestApplyItemSetInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  domain.ApplyItemSetRequest
	}{
		{
			name: "champion must be > 0",
			req: domain.ApplyItemSetRequest{
				Position:   domain.Top,
				ChampionID: 0,
				Blocks: []domain.ApplyItemSetBlock{
					{Type: "Starter", ItemIDs: []int{1055, 3006}},
				},
			},
		},
		{
			name: "position must be valid",
			req: domain.ApplyItemSetRequest{
				Position:   "invalid",
				ChampionID: 240,
				Blocks: []domain.ApplyItemSetBlock{
					{Type: "Starter", ItemIDs: []int{1055, 3006}},
				},
			},
		},
		{
			name: "requires at least one block",
			req: domain.ApplyItemSetRequest{
				Position:   domain.Top,
				ChampionID: 240,
				Blocks:     []domain.ApplyItemSetBlock{},
			},
		},
		{
			name: "requires at least one item id",
			req: domain.ApplyItemSetRequest{
				Position:   domain.Top,
				ChampionID: 240,
				Blocks: []domain.ApplyItemSetBlock{
					{Type: "Starter", ItemIDs: []int{}},
				},
			},
		},
		{
			name: "block type is required",
			req: domain.ApplyItemSetRequest{
				Position:   domain.Top,
				ChampionID: 240,
				Blocks: []domain.ApplyItemSetBlock{
					{Type: "", ItemIDs: []int{1055, 3006}},
				},
			},
		},
		{
			name: "item ids must be > 0",
			req: domain.ApplyItemSetRequest{
				Position:   domain.Top,
				ChampionID: 240,
				Blocks: []domain.ApplyItemSetBlock{
					{Type: "Starter", ItemIDs: []int{1055, 0}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
			client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

			err := client.ApplyItemSet(context.Background(), tt.req)
			if !errors.Is(err, ErrInvalidItemSetRequest) {
				t.Fatalf("expected ErrInvalidItemSetRequest, got %v", err)
			}
		})
	}
}

func TestApplyItemSetSuccessUpsertsManagedSet(t *testing.T) {
	t.Parallel()

	type collection struct {
		Timestamp uint64            `json:"timestamp"`
		AccountID uint64            `json:"accountId"`
		ItemSets  []json.RawMessage `json:"itemSets"`
	}

	var (
		putCalled bool
		gotPut    collection
	)

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:secret"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedAuth {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}

		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-summoner/v1/current-summoner":
			_, _ = fmt.Fprint(w, `{"summonerId":321,"accountId":654}`)
		case "/lol-item-sets/v1/item-sets/321/sets":
			if r.Method == http.MethodGet {
				_, _ = fmt.Fprint(w, `{"timestamp":42,"accountId":654,"itemSets":[{"uid":"user-set-1","title":"User Set","mode":"any","map":"any","type":"custom","sortrank":5,"startedFrom":"blank","associatedChampions":[],"associatedMaps":[11],"blocks":[],"preferredItemSlots":[]},{"uid":"lol-autobuild:240:support","title":"Old Auto","mode":"any","map":"any","type":"custom","sortrank":0,"startedFrom":"blank","associatedChampions":[240],"associatedMaps":[11],"blocks":[],"preferredItemSlots":[]}]}`)
				return
			}

			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT item-sets, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotPut); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			putCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Position:   domain.Support,
		Patch:      "16.7",
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006, 1055}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyItemSet() error = %v", err)
	}
	if !putCalled {
		t.Fatalf("expected item set PUT call")
	}
	if gotPut.AccountID != 654 {
		t.Fatalf("expected accountId 654, got %d", gotPut.AccountID)
	}
	if gotPut.Timestamp != 42 {
		t.Fatalf("expected timestamp 42, got %d", gotPut.Timestamp)
	}
	if len(gotPut.ItemSets) != 2 {
		t.Fatalf("expected 2 item sets after upsert, got %d", len(gotPut.ItemSets))
	}

	uids := make([]string, 0, len(gotPut.ItemSets))
	var managed itemSet
	for _, raw := range gotPut.ItemSets {
		var parsed itemSet
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("unmarshal set: %v", err)
		}
		uids = append(uids, parsed.UID)
		if parsed.UID == "lol-autobuild:240:support" {
			managed = parsed
		}
	}

	managedCount := 0
	for _, uid := range uids {
		if uid == "lol-autobuild:240:support" {
			managedCount++
		}
	}
	if managedCount != 1 {
		t.Fatalf("expected exactly one managed set after upsert, got %d (%v)", managedCount, uids)
	}

	if managed.Title != "AutoBuild 240 support 16.7" {
		t.Fatalf("unexpected managed title: %q", managed.Title)
	}
	if len(managed.Blocks) != 1 || managed.Blocks[0].Type != "Starter" {
		t.Fatalf("unexpected managed blocks: %#v", managed.Blocks)
	}
	if len(managed.Blocks[0].Items) != 2 {
		t.Fatalf("expected deduped 2 items, got %#v", managed.Blocks[0].Items)
	}
	if managed.Blocks[0].Items[0].ID != "1055" || managed.Blocks[0].Items[1].ID != "3006" {
		t.Fatalf("unexpected managed items: %#v", managed.Blocks[0].Items)
	}
}

func TestApplyItemSetPreservesOrderedBlocksAndEmptyBlocks(t *testing.T) {
	t.Parallel()

	type collection struct {
		Timestamp uint64            `json:"timestamp"`
		AccountID uint64            `json:"accountId"`
		ItemSets  []json.RawMessage `json:"itemSets"`
	}

	var gotPut collection

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:secret"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedAuth {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}

		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-summoner/v1/current-summoner":
			_, _ = fmt.Fprint(w, `{"summonerId":321,"accountId":654}`)
		case "/lol-item-sets/v1/item-sets/321/sets":
			if r.Method == http.MethodGet {
				_, _ = fmt.Fprint(w, `{"timestamp":42,"accountId":654,"itemSets":[]}`)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&gotPut); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Position:   domain.Support,
		Patch:      "16.7",
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006, 1055}},
			{Type: "1st Item", ItemIDs: []int{}},
			{Type: "Boots", ItemIDs: []int{3006}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyItemSet() error = %v", err)
	}

	if len(gotPut.ItemSets) != 1 {
		t.Fatalf("expected 1 item set after upsert, got %d", len(gotPut.ItemSets))
	}

	var managed itemSet
	if err := json.Unmarshal(gotPut.ItemSets[0], &managed); err != nil {
		t.Fatalf("unmarshal managed set: %v", err)
	}

	if len(managed.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %#v", managed.Blocks)
	}
	if managed.Blocks[0].Type != "Starter" || managed.Blocks[1].Type != "1st Item" || managed.Blocks[2].Type != "Boots" {
		t.Fatalf("unexpected block order/types: %#v", managed.Blocks)
	}
	if len(managed.Blocks[0].Items) != 2 || managed.Blocks[0].Items[0].ID != "1055" || managed.Blocks[0].Items[1].ID != "3006" {
		t.Fatalf("unexpected starter block items: %#v", managed.Blocks[0].Items)
	}
	if len(managed.Blocks[1].Items) != 0 {
		t.Fatalf("expected empty block to be preserved, got %#v", managed.Blocks[1].Items)
	}
	if len(managed.Blocks[2].Items) != 1 || managed.Blocks[2].Items[0].ID != "3006" {
		t.Fatalf("unexpected boots block items: %#v", managed.Blocks[2].Items)
	}
}

func TestApplyItemSetFallsBackWhenProcessCandidateFails(t *testing.T) {
	t.Parallel()

	autoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":157,"spell1Id":4,"spell2Id":14}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoServer.Close()

	fallbackPutCalled := false
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":2,"myTeam":[{"cellId":2,"championId":240,"spell1Id":4,"spell2Id":14}]}`)
		case "/lol-summoner/v1/current-summoner":
			_, _ = fmt.Fprint(w, `{"summonerId":321,"accountId":654}`)
		case "/lol-item-sets/v1/item-sets/321/sets":
			if r.Method == http.MethodGet {
				_, _ = fmt.Fprint(w, `{"timestamp":1,"accountId":654,"itemSets":[]}`)
				return
			}
			fallbackPutCalled = true
			w.WriteHeader(http.StatusNoContent)
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

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Position:   domain.Support,
		Patch:      "16.7",
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyItemSet() error = %v", err)
	}
	if !fallbackPutCalled {
		t.Fatalf("expected fallback candidate to apply item set")
	}
}

func TestApplyItemSetAllCandidatesFailRespectsPriority(t *testing.T) {
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

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Position:   domain.Support,
		Patch:      "16.7",
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006}},
		},
	})
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged priority, got %v", err)
	}
}
func TestApplyItemSetFailsWhenChampionIsNotSelected(t *testing.T) {
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

	err := client.ApplyItemSet(context.Background(), domain.ApplyItemSetRequest{
		ChampionID: 240,
		Position:   domain.Support,
		Patch:      "16.7",
		Blocks: []domain.ApplyItemSetBlock{
			{Type: "Starter", ItemIDs: []int{1055, 3006}},
		},
	})
	if !errors.Is(err, ErrChampionNotSelected) {
		t.Fatalf("expected ErrChampionNotSelected priority, got %v", err)
	}
}
