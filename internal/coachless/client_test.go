package coachless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func TestGetPatchesParsesResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ChampionWinprob/GetPatches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatalf("missing authorization header")
		}

		_ = json.NewEncoder(w).Encode([]ports.PatchInfo{{
			Label:      "16.7",
			Major:      16,
			Patch:      7,
			MatchCount: 123,
		}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	patches, err := client.GetPatches(context.Background(), "token")
	if err != nil {
		t.Fatalf("GetPatches() error = %v", err)
	}

	if len(patches) != 1 || patches[0].Label != "16.7" {
		t.Fatalf("unexpected patches: %#v", patches)
	}
}

func TestGetKeystoneDataSendsBody(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body ports.KeystoneRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if len(body.CommonFilters.ChampionIDs) == 0 || body.CommonFilters.ChampionIDs[0] != 240 {
			t.Fatalf("unexpected body: %#v", body)
		}

		_ = json.NewEncoder(w).Encode([]ports.KeystoneStat{{
			Rune:       8010,
			WPAOverall: 1.2,
			Occurrence: 1000,
		}})
	}))
	defer srv.Close()

	var (
		client = NewClient(srv.URL, 2*time.Second)
		req    = ports.KeystoneRequest{
			CommonFilters: ports.CommonFilters{
				ChampionIDs: []int{240},
				Role:        0,
				Patch: ports.PatchFilter{
					Major:          16,
					Patch:          7,
					PatchAdditions: 2,
				},
				LeagueTiers: []int{5, 6, 7},
			},
		}
	)

	stats, err := client.GetKeystoneData(context.Background(), "token", req)
	if err != nil {
		t.Fatalf("GetKeystoneData() error = %v", err)
	}

	if !called {
		t.Fatal("expected handler call")
	}

	if len(stats) != 1 || stats[0].Rune != 8010 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestGetSecondaryTreePlaycountSendsBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/Rune/GetSecondaryTreePlaycount" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body ports.SecondaryTreePlaycountRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Tree != ports.RuneStylePrecision || body.Keystone != 8005 || body.CommonFilters.Role != 3 {
			t.Fatalf("unexpected body: %#v", body)
		}

		_ = json.NewEncoder(w).Encode([]ports.RuneTreePlaycount{
			{Tree: ports.RuneStyleSorcery, Occurrence: 1234},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	stats, err := client.GetSecondaryTreePlaycount(context.Background(), "token", ports.SecondaryTreePlaycountRequest{
		CommonFilters: ports.CommonFilters{Role: 3},
		Tree:          ports.RuneStylePrecision,
		Keystone:      8005,
	})
	if err != nil {
		t.Fatalf("GetSecondaryTreePlaycount() error = %v", err)
	}
	if len(stats) != 1 || stats[0].Tree != ports.RuneStyleSorcery || stats[0].Occurrence != 1234 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestGetRuneStatsForKeystoneAndTreeSendsBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/Rune/GetRunesForKeystoneAndTree" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body ports.RuneStatsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Keystone != 8437 || body.MainTree != ports.RuneStyleResolve || body.TreeToLoad != ports.RuneStyleSorcery {
			t.Fatalf("unexpected body: %#v", body)
		}

		_ = json.NewEncoder(w).Encode(ports.RuneStatsByRow{
			RowOnes:   []ports.RuneStat{{Rune: 8224, WPAOverall: 0.6, Occurrence: 100}},
			RowTwos:   []ports.RuneStat{{Rune: 8233, WPAOverall: 0.7, Occurrence: 200}},
			RowThrees: []ports.RuneStat{{Rune: 8237, WPAOverall: 0.8, Occurrence: 300}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	stats, err := client.GetRuneStatsForKeystoneAndTree(context.Background(), "token", ports.RuneStatsRequest{
		Keystone:   8437,
		MainTree:   ports.RuneStyleResolve,
		TreeToLoad: ports.RuneStyleSorcery,
	})
	if err != nil {
		t.Fatalf("GetRuneStatsForKeystoneAndTree() error = %v", err)
	}
	if len(stats.RowOnes) != 1 || stats.RowOnes[0].Rune != 8224 || len(stats.RowTwos) != 1 || stats.RowTwos[0].Rune != 8233 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestGetShardStatsForKeystoneAndTreeSendsBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/Rune/GetShardsForKeystoneAndTree" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body ports.ShardStatsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Keystone != 8437 || body.CommonFilters.Role != 4 {
			t.Fatalf("unexpected body: %#v", body)
		}

		_ = json.NewEncoder(w).Encode(ports.ShardStats{
			Offense: []ports.RuneStat{{Rune: 5008, WPAOverall: 0.5, Occurrence: 100}},
			Flex:    []ports.RuneStat{{Rune: 5008, WPAOverall: 0.4, Occurrence: 100}},
			Defense: []ports.RuneStat{{Rune: 5002, WPAOverall: 0.3, Occurrence: 100}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	stats, err := client.GetShardStatsForKeystoneAndTree(context.Background(), "token", ports.ShardStatsRequest{
		CommonFilters: ports.CommonFilters{Role: 4},
		Keystone:      8437,
	})
	if err != nil {
		t.Fatalf("GetShardStatsForKeystoneAndTree() error = %v", err)
	}
	if len(stats.Offense) != 1 || stats.Offense[0].Rune != 5008 || len(stats.Defense) != 1 || stats.Defense[0].Rune != 5002 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestRefreshParsesTokens(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/Auth/refresh" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access",
			"refreshToken": "refresh",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	pair, err := client.Refresh(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if pair.AccessToken != "access" || pair.RefreshToken != "refresh" {
		t.Fatalf("unexpected pair: %#v", pair)
	}
}
