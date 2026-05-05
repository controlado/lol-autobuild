package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type trackingRefresher struct {
	refreshed domain.TokenPair
	err       error
	calls     int
	gotToken  string
}

func (r *trackingRefresher) Refresh(_ context.Context, refreshToken string) (domain.TokenPair, error) {
	r.calls++
	r.gotToken = refreshToken
	if r.err != nil {
		return domain.TokenPair{}, r.err
	}

	return r.refreshed, nil
}

type trackingAutoSource struct {
	pair  domain.TokenPair
	err   error
	calls int
}

func (a *trackingAutoSource) Acquire(_ context.Context) (domain.TokenPair, error) {
	a.calls++
	if a.err != nil {
		return domain.TokenPair{}, a.err
	}

	return a.pair, nil
}

func TestCoachlessSessionExport(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		store   *fakeStore
		wantErr bool
	}{
		{
			name:    "missing tokens",
			store:   &fakeStore{readErr: domain.ErrTokensNotFound},
			wantErr: true,
		},
		{
			name: "valid bundle",
			store: &fakeStore{pair: domain.TokenPair{
				AccessToken:  testJWT(`{"exp":1777896000,"isSubscribed":"1"}`),
				RefreshToken: "refresh",
				ExpiresAt:    expiresAt,
			}},
		},
		{
			name: "missing refresh",
			store: &fakeStore{pair: domain.TokenPair{
				AccessToken: testJWT(`{"exp":1777896000,"isSubscribed":"1"}`),
				ExpiresAt:   expiresAt,
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := NewCoachlessSession(fakeTokenRefresher{}, tt.store, nil, CoachlessSessionOptions{})
			raw, err := session.Export(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatal("Export() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			var bundle CoachlessAuthBundle
			if err := json.Unmarshal([]byte(raw), &bundle); err != nil {
				t.Fatalf("decode bundle: %v", err)
			}
			if bundle.Type != CoachlessAuthBundleType || bundle.Version != CoachlessAuthBundleVersion {
				t.Fatalf("bundle identity = %q/%d", bundle.Type, bundle.Version)
			}
			if bundle.AccessToken == "" || bundle.RefreshToken != "refresh" {
				t.Fatalf("bundle tokens = %#v", bundle)
			}
			if bundle.AccessTokenExpiresAt == nil || !bundle.AccessTokenExpiresAt.Equal(expiresAt) {
				t.Fatalf("bundle expires_at = %v, want %v", bundle.AccessTokenExpiresAt, expiresAt)
			}
			if !strings.Contains(raw, "\n  \"refresh_token\"") {
				t.Fatalf("bundle should be formatted JSON, got %q", raw)
			}
		})
	}
}

func TestCoachlessSessionImport(t *testing.T) {
	t.Parallel()

	futureExp := time.Now().Add(30 * time.Minute).Unix()
	refresher := &trackingRefresher{
		refreshed: domain.TokenPair{
			AccessToken:  testJWT(`{"exp":` + intString(futureExp) + `,"isSubscribed":"0"}`),
			RefreshToken: "next-refresh",
		},
	}
	store := &fakeStore{}
	session := NewCoachlessSession(refresher, store, nil, CoachlessSessionOptions{})

	raw := `{
		"type":"lol-autobuild.coachless.auth",
		"version":1,
		"access_token":"ignored-access",
		"refresh_token":" imported-refresh "
	}`
	if err := session.Import(context.Background(), raw); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if refresher.calls != 1 || refresher.gotToken != "imported-refresh" {
		t.Fatalf("refresh calls/token = %d/%q", refresher.calls, refresher.gotToken)
	}
	if store.writeCalls != 1 {
		t.Fatalf("WriteTokens calls = %d, want 1", store.writeCalls)
	}
	if store.pair.AccessToken != refresher.refreshed.AccessToken || store.pair.RefreshToken != "next-refresh" {
		t.Fatalf("stored pair = %#v", store.pair)
	}
	if store.pair.ExpiresAt.IsZero() {
		t.Fatal("stored ExpiresAt should be derived from refreshed access token")
	}
}

func TestCoachlessSessionImportRejectsInvalidBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing refresh",
			raw:  `{"type":"lol-autobuild.coachless.auth","version":1,"access_token":"access"}`,
		},
		{
			name: "invalid type",
			raw:  `{"type":"other","version":1,"refresh_token":"refresh"}`,
		},
		{
			name: "invalid version",
			raw:  `{"type":"lol-autobuild.coachless.auth","version":2,"refresh_token":"refresh"}`,
		},
		{
			name: "malformed json",
			raw:  `{`,
		},
		{
			name: "unknown field",
			raw:  `{"type":"lol-autobuild.coachless.auth","version":1,"refresh_token":"refresh","extra":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			refresher := &trackingRefresher{}
			store := &fakeStore{}
			session := NewCoachlessSession(refresher, store, nil, CoachlessSessionOptions{})

			if err := session.Import(context.Background(), tt.raw); err == nil {
				t.Fatal("Import() error = nil, want error")
			}
			if refresher.calls != 0 {
				t.Fatalf("Refresh calls = %d, want 0", refresher.calls)
			}
			if store.writeCalls != 0 {
				t.Fatalf("WriteTokens calls = %d, want 0", store.writeCalls)
			}
		})
	}
}

func TestCoachlessSessionLogoutClearsStore(t *testing.T) {
	t.Parallel()

	store := &fakeStore{pair: domain.TokenPair{AccessToken: "access", RefreshToken: "refresh"}}
	session := NewCoachlessSession(fakeTokenRefresher{}, store, nil, CoachlessSessionOptions{})

	if err := session.Logout(context.Background()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if store.clearCalls != 1 {
		t.Fatalf("ClearTokens calls = %d, want 1", store.clearCalls)
	}
	if store.pair != (domain.TokenPair{}) {
		t.Fatalf("store pair = %#v, want empty", store.pair)
	}
}

func TestCoachlessSessionStatusDoesNotAcquireOrRefresh(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(30 * time.Minute).Unix()
	store := &fakeStore{pair: domain.TokenPair{
		AccessToken:  testJWT(`{"exp":` + intString(exp) + `,"isSubscribed":"1"}`),
		RefreshToken: "refresh",
	}}
	auto := &trackingAutoSource{}
	refresher := &trackingRefresher{err: errors.New("should not refresh")}
	session := NewCoachlessSession(refresher, store, auto, CoachlessSessionOptions{TokenSkew: 30 * time.Second})

	status := session.Status(context.Background())
	if status.Status != CoachlessSessionStatusStored || status.Plan != CoachlessPlanPremium {
		t.Fatalf("Status() = %+v", status)
	}
	if status.ExpiresAt == nil {
		t.Fatal("Status().ExpiresAt = nil, want value")
	}
	if auto.calls != 0 {
		t.Fatalf("Acquire calls = %d, want 0", auto.calls)
	}
	if refresher.calls != 0 {
		t.Fatalf("Refresh calls = %d, want 0", refresher.calls)
	}
}

func TestCoachlessSessionStatusHandlesMissingAndExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pair domain.TokenPair
		err  error
		want CoachlessSessionStatus
	}{
		{
			name: "missing",
			err:  domain.ErrTokensNotFound,
			want: CoachlessSessionStatusMissing,
		},
		{
			name: "expired",
			pair: domain.TokenPair{
				AccessToken:  testJWT(`{"exp":1,"isSubscribed":"0"}`),
				RefreshToken: "refresh",
			},
			want: CoachlessSessionStatusExpired,
		},
		{
			name: "invalid without refresh",
			pair: domain.TokenPair{AccessToken: "not-a-jwt"},
			want: CoachlessSessionStatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status := NewCoachlessSession(fakeTokenRefresher{}, &fakeStore{pair: tt.pair, readErr: tt.err}, nil, CoachlessSessionOptions{}).Status(context.Background())
			if status.Status != tt.want {
				t.Fatalf("Status() = %+v, want status %q", status, tt.want)
			}
		})
	}
}

func intString(value int64) string {
	return strconv.FormatInt(value, 10)
}
