package lcu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestApplyRunePageReturnsNotConfiguredWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(false, "")
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestApplyRunePageDryRunSkipsIO(t *testing.T) {
	t.Parallel()

	client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

	req := validRunePageApplyRequest()
	req.DryRun = true
	if err := client.ApplyRunePage(context.Background(), req); err != nil {
		t.Fatalf("expected nil error in dry-run, got %v", err)
	}
}

func TestApplyRunePageInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  domain.ApplyRunePageRequest
	}{
		{
			name: "champion must be > 0",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.ChampionID = 0
				return req
			}(),
		},
		{
			name: "position must be valid",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.Position = "invalid"
				return req
			}(),
		},
		{
			name: "primary style must be valid",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.Page.PrimaryStyleID = 9999
				return req
			}(),
		},
		{
			name: "styles must be distinct",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.Page.SubStyleID = req.Page.PrimaryStyleID
				return req
			}(),
		},
		{
			name: "requires nine perks",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.Page.SelectedPerkIDs = req.Page.SelectedPerkIDs[:8]
				return req
			}(),
		},
		{
			name: "perk ids must be positive",
			req: func() domain.ApplyRunePageRequest {
				req := validRunePageApplyRequest()
				req.Page.SelectedPerkIDs[3] = 0
				return req
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(true, filepath.Join(t.TempDir(), "missing-lockfile"))
			client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }

			err := client.ApplyRunePage(context.Background(), tt.req)
			if !errors.Is(err, ErrInvalidRunePageRequest) {
				t.Fatalf("expected ErrInvalidRunePageRequest, got %v", err)
			}
		})
	}
}

func TestApplyRunePageSuccessCreatesPageWithoutDeletingUserPage(t *testing.T) {
	t.Parallel()

	var (
		calls              []string
		createPayload      runePageCreateRequest
		expectedPerkIDs    = validRunePageApplyRequest().Page.SelectedPerkIDs
		expectedPageName   = "[autobuild] [top] Kled"
		expectedPrimary    = domain.RuneStylePrecision
		expectedSecondary  = domain.RuneStyleSorcery
		expectedCurrentOld = []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            "User Page",
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: expectedCurrentOld,
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			t.Fatalf("unexpected delete of user page")
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{
						ID:              123,
						Current:         true,
						IsDeletable:     true,
						Name:            "User Page",
						PrimaryStyleID:  domain.RuneStyleResolve,
						SubStyleID:      domain.RuneStyleSorcery,
						SelectedPerkIDs: expectedCurrentOld,
					},
				})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}

	if createPayload.Name != expectedPageName || createPayload.PrimaryStyleID != expectedPrimary || createPayload.SubStyleID != expectedSecondary || !createPayload.Current {
		t.Fatalf("unexpected create payload: %#v", createPayload)
	}
	if !slices.Equal(createPayload.SelectedPerkIDs, expectedPerkIDs) {
		t.Fatalf("selectedPerkIds = %#v, want %#v", createPayload.SelectedPerkIDs, expectedPerkIDs)
	}
}

func TestApplyRunePageReusesExistingManagedPageWhenCurrentIsUserPage(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            "User Page",
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002},
			})
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{ID: 123, Current: true, IsDeletable: true, Name: "User Page"},
					{ID: 456, IsDeletable: true, Name: "AutoBuild 777 mid"},
				})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		case "/lol-perks/v1/pages/123":
			t.Fatalf("unexpected delete of user page")
		case "/lol-perks/v1/pages/456":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE managed page, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"DELETE /lol-perks/v1/pages/456",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestApplyRunePageRestoresNonCurrentManagedPageWithoutSelectingItWhenCreateFails(t *testing.T) {
	t.Parallel()

	var (
		postCount      int
		restorePayload runePageCreateRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{ID: 123, Current: true, IsDeletable: true, Name: "User Page"})
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{
						ID:              456,
						Current:         false,
						IsDeletable:     true,
						Name:            "AutoBuild 777 mid",
						PrimaryStyleID:  domain.RuneStyleResolve,
						SubStyleID:      domain.RuneStyleSorcery,
						SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002},
					},
				})
				return
			}
			postCount++
			if postCount == 1 {
				http.Error(w, "create failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&restorePayload); err != nil {
				t.Fatalf("decode restore payload: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		case "/lol-perks/v1/pages/456":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE managed page, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if postCount != 2 {
		t.Fatalf("expected create plus restore calls, got %d", postCount)
	}
	if restorePayload.Current {
		t.Fatalf("expected restored non-current managed page to stay non-current")
	}
}

func TestApplyRunePageIgnoresNonDeletableManagedPageAndCreatesNew(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{ID: 123, Current: true, IsDeletable: true, Name: "User Page"})
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{ID: 456, IsDeletable: false, Name: "AutoBuild 777 mid"},
				})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		case "/lol-perks/v1/pages/456":
			t.Fatalf("unexpected delete of non-deletable managed page")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestApplyRunePageReusesFirstDeletableManagedPage(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{ID: 123, Current: true, IsDeletable: true, Name: "User Page"})
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{ID: 456, IsDeletable: true, Name: "AutoBuild 777 mid"},
					{ID: 789, IsDeletable: true, Name: "AutoBuild 240 top"},
				})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		case "/lol-perks/v1/pages/456":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE first managed page, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		case "/lol-perks/v1/pages/789":
			t.Fatalf("unexpected delete of second managed page")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"DELETE /lol-perks/v1/pages/456",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestApplyRunePageCreatesPageWhenCurrentPageIsMissing(t *testing.T) {
	t.Parallel()

	var (
		calls         []string
		createPayload runePageCreateRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			http.NotFound(w, r)
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
	if !createPayload.Current {
		t.Fatalf("expected created page to be current")
	}
}

func TestApplyRunePageReusesManagedPageWhenCurrentPageIsMissing(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			http.NotFound(w, r)
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{ID: 456, IsDeletable: true, Name: "AutoBuild 777 mid"},
				})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		case "/lol-perks/v1/pages/456":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE managed page, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"GET /lol-perks/v1/pages",
		"DELETE /lol-perks/v1/pages/456",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestApplyRunePageSuccessReplacesManagedPage(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            "AutoBuild 240 top",
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002},
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE page, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		case "/lol-perks/v1/pages":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	if err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest()); err != nil {
		t.Fatalf("ApplyRunePage() error = %v", err)
	}

	wantCalls := []string{
		"GET /lol-champ-select/v1/session",
		"GET /lol-perks/v1/currentpage",
		"DELETE /lol-perks/v1/pages/123",
		"POST /lol-perks/v1/pages",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestApplyRunePageReturnsCreateErrorForUserPage(t *testing.T) {
	t.Parallel()

	var postCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            "User Page",
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002},
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			t.Fatalf("unexpected delete of user page")
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{
					{ID: 123, Current: true, IsDeletable: true, Name: "User Page"},
				})
				return
			}
			postCalled = true
			http.Error(w, "invalid rune page", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if !postCalled {
		t.Fatalf("expected create endpoint to be called")
	}
	if !strings.Contains(err.Error(), "create rune page failed LCU validation") {
		t.Fatalf("expected create validation context, got %v", err)
	}
	if errors.Is(err, domain.ErrRunePageLimitReached) {
		t.Fatalf("unexpected ErrRunePageLimitReached for generic create failure")
	}
}

func TestApplyRunePageReturnsLimitReachedWhenCreateHitsPageCap(t *testing.T) {
	t.Parallel()

	var postCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			http.NotFound(w, r)
		case "/lol-perks/v1/pages":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]runePage{})
				return
			}
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST pages, got %s", r.Method)
			}
			postCalled = true
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"errorCode":"RPC_ERROR","httpStatus":400,"implementationDetails":{},"message":"Max pages reached"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if !errors.Is(err, domain.ErrRunePageLimitReached) {
		t.Fatalf("expected ErrRunePageLimitReached, got %v", err)
	}
	if !postCalled {
		t.Fatalf("expected create endpoint to be called")
	}
}

func TestApplyRunePageFailsWhenCurrentPageIsNotDeletable(t *testing.T) {
	t.Parallel()

	var deleteCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     false,
				Name:            "AutoBuild 240 top",
				PrimaryStyleID:  domain.RuneStylePrecision,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: validRunePageApplyRequest().Page.SelectedPerkIDs,
			})
		case "/lol-perks/v1/pages/123", "/lol-perks/v1/pages":
			deleteCalled = true
			http.Error(w, "unexpected mutation", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if deleteCalled {
		t.Fatalf("expected no delete or create call for non-deletable current page")
	}
}

func TestApplyRunePageFailsWhenChampionChanges(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":157}]}`)
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrChampionSelectionChanged) {
		t.Fatalf("expected ErrChampionSelectionChanged, got %v", err)
	}
}

func TestApplyRunePageRestoresPreviousPageWhenCreateFails(t *testing.T) {
	t.Parallel()

	var (
		postCount       int
		restorePayload  runePageCreateRequest
		restorePerkIDs  = []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002}
		restoredOldName = "AutoBuild 240 top"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            restoredOldName,
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: restorePerkIDs,
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			w.WriteHeader(http.StatusNoContent)
		case "/lol-perks/v1/pages":
			postCount++
			if postCount == 1 {
				http.Error(w, "create failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&restorePayload); err != nil {
				t.Fatalf("decode restore payload: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if postCount != 2 {
		t.Fatalf("expected create plus restore calls, got %d", postCount)
	}
	if restorePayload.Name != restoredOldName || !restorePayload.Current || restorePayload.PrimaryStyleID != domain.RuneStyleResolve {
		t.Fatalf("unexpected restore payload: %#v", restorePayload)
	}
	if !slices.Equal(restorePayload.SelectedPerkIDs, restorePerkIDs) {
		t.Fatalf("restore perks = %#v, want %#v", restorePayload.SelectedPerkIDs, restorePerkIDs)
	}
	if !strings.Contains(err.Error(), "previous rune page restored") {
		t.Fatalf("expected restore context in error, got %v", err)
	}
}

func TestApplyRunePageRestoresPreviousPageWhenApplyContextIsCanceledAfterDelete(t *testing.T) {
	t.Parallel()

	var (
		postCount       int32
		restorePayload  runePageCreateRequest
		restorePerkIDs  = []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002}
		restoredOldName = "AutoBuild 240 top"
	)

	applyCtx, cancelApply := context.WithCancel(context.Background())
	defer cancelApply()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            restoredOldName,
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: restorePerkIDs,
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			w.WriteHeader(http.StatusNoContent)
		case "/lol-perks/v1/pages":
			switch atomic.AddInt32(&postCount, 1) {
			case 1:
				cancelApply()
				http.Error(w, "create failed", http.StatusInternalServerError)
				return
			case 2:
				if err := json.NewDecoder(r.Body).Decode(&restorePayload); err != nil {
					t.Fatalf("decode restore payload: %v", err)
				}
				w.WriteHeader(http.StatusCreated)
			default:
				t.Fatalf("unexpected extra create call")
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(applyCtx, validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if atomic.LoadInt32(&postCount) != 2 {
		t.Fatalf("expected create plus restore calls, got %d", postCount)
	}
	if restorePayload.Name != restoredOldName || !restorePayload.Current || restorePayload.PrimaryStyleID != domain.RuneStyleResolve {
		t.Fatalf("unexpected restore payload: %#v", restorePayload)
	}
	if !slices.Equal(restorePayload.SelectedPerkIDs, restorePerkIDs) {
		t.Fatalf("restore perks = %#v, want %#v", restorePayload.SelectedPerkIDs, restorePerkIDs)
	}
	if !strings.Contains(err.Error(), "previous rune page restored") {
		t.Fatalf("expected restore context in error, got %v", err)
	}
}

func TestApplyRunePageReportsRestoreFailureWhenCreateAndRestoreFail(t *testing.T) {
	t.Parallel()

	var postCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-champ-select/v1/session":
			_, _ = fmt.Fprint(w, `{"queueId":420,"localPlayerCellId":1,"myTeam":[{"cellId":1,"championId":240}]}`)
		case "/lol-perks/v1/currentpage":
			_ = json.NewEncoder(w).Encode(runePage{
				ID:              123,
				Current:         true,
				IsDeletable:     true,
				Name:            "AutoBuild 240 top",
				PrimaryStyleID:  domain.RuneStyleResolve,
				SubStyleID:      domain.RuneStyleSorcery,
				SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8237, 5008, 5008, 5002},
			})
		case "/lol-perks/v1/pages/validate":
			t.Fatalf("unexpected validate endpoint call")
		case "/lol-perks/v1/pages/123":
			w.WriteHeader(http.StatusNoContent)
		case "/lol-perks/v1/pages":
			postCount++
			http.Error(w, "post failed", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestRunePageClient(t, server)
	err := client.ApplyRunePage(context.Background(), validRunePageApplyRequest())
	if !errors.Is(err, ErrRunePageApplyFailed) {
		t.Fatalf("expected ErrRunePageApplyFailed, got %v", err)
	}
	if postCount != 2 {
		t.Fatalf("expected create plus failed restore calls, got %d", postCount)
	}
	if !strings.Contains(err.Error(), "restore previous rune page failed") {
		t.Fatalf("expected restore failure context in error, got %v", err)
	}
}

func validRunePageApplyRequest() domain.ApplyRunePageRequest {
	return domain.ApplyRunePageRequest{
		ChampionID:   240,
		ChampionName: "Kled",
		Position:     domain.Top,
		Page: domain.RunePage{
			PrimaryStyleID:  domain.RuneStylePrecision,
			SubStyleID:      domain.RuneStyleSorcery,
			SelectedPerkIDs: []int{8005, 9101, 9104, 8299, 8233, 8237, 5005, 5008, 5002},
		},
	}
}

func newTestRunePageClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	lockfilePath := filepath.Join(t.TempDir(), "lockfile")
	writeLockfile(t, lockfilePath, mustServerPort(t, server.URL))

	client := NewClient(true, lockfilePath)
	client.discoverProcessConnections = func(context.Context) []connectionCandidate { return nil }
	return client
}
