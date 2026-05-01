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
