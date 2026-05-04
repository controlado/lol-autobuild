package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestAccessTokenRefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	var (
		store = &fakeStore{
			pair: domain.TokenPair{
				AccessToken:  "expired",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(-1 * time.Minute),
			},
		}
		p = NewProvider(
			fakeTokenRefresher{
				refreshed: domain.TokenPair{
					AccessToken:  "new-access",
					RefreshToken: "new-refresh",
					ExpiresAt:    time.Now().Add(30 * time.Minute),
				},
			},
			store,
			nil,
			nil,
			ProviderOptions{TokenSkew: 30 * time.Second},
		)
	)

	token, err := p.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}

	if token != "new-access" {
		t.Fatalf("unexpected token: %s", token)
	}

	if store.pair.RefreshToken != "new-refresh" {
		t.Fatalf("expected stored refresh token to update")
	}
}

func TestAccessTokenDoesNotPersistInvalidRefresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		refreshed domain.TokenPair
	}{
		{
			name:      "missing access token",
			refreshed: domain.TokenPair{RefreshToken: "new-refresh"},
		},
		{
			name:      "missing refresh token",
			refreshed: domain.TokenPair{AccessToken: "new-access"},
		},
		{
			name: "blank tokens",
			refreshed: domain.TokenPair{
				AccessToken:  " ",
				RefreshToken: "\t",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stored := domain.TokenPair{
				AccessToken:  "expired",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(-1 * time.Minute),
			}
			store := &fakeStore{pair: stored}
			p := NewProvider(
				fakeTokenRefresher{refreshed: tt.refreshed},
				store,
				nil,
				nil,
				ProviderOptions{TokenSkew: 30 * time.Second},
			)

			_, err := p.AccessToken(context.Background())
			if !errors.Is(err, ErrAccessTokenUnavailable) {
				t.Fatalf("AccessToken() error = %v, want %v", err, ErrAccessTokenUnavailable)
			}
			if store.writeCalls != 0 {
				t.Fatalf("WriteTokens calls = %d, want 0", store.writeCalls)
			}
			if store.pair != stored {
				t.Fatalf("stored pair changed to %#v, want %#v", store.pair, stored)
			}
		})
	}
}

func TestRefreshDoesNotPersistInvalidTokenPair(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		refreshed domain.TokenPair
	}{
		{
			name:      "missing access token",
			refreshed: domain.TokenPair{RefreshToken: "new-refresh"},
		},
		{
			name:      "missing refresh token",
			refreshed: domain.TokenPair{AccessToken: "new-access"},
		},
		{
			name: "blank tokens",
			refreshed: domain.TokenPair{
				AccessToken:  " ",
				RefreshToken: "\t",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stored := domain.TokenPair{
				AccessToken:  "expired",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(-1 * time.Minute),
			}
			store := &fakeStore{pair: stored}
			p := NewProvider(
				fakeTokenRefresher{refreshed: tt.refreshed},
				store,
				nil,
				nil,
				ProviderOptions{TokenSkew: 30 * time.Second},
			)

			if _, err := p.Refresh(context.Background()); err == nil {
				t.Fatal("expected Refresh() error")
			}
			if store.writeCalls != 0 {
				t.Fatalf("WriteTokens calls = %d, want 0", store.writeCalls)
			}
			if store.pair != stored {
				t.Fatalf("stored pair changed to %#v, want %#v", store.pair, stored)
			}
		})
	}
}

func TestAccessTokenFallsBackToManual(t *testing.T) {
	t.Parallel()

	var (
		p = NewProvider(
			fakeTokenRefresher{},
			&fakeStore{readErr: errors.New("not found")},
			fakeAutoSource{err: errors.New("auto fail")},
			fakeManualSource{
				pair: domain.TokenPair{
					AccessToken: "manual-access",
					ExpiresAt:   time.Now().Add(15 * time.Minute),
				},
			},
			ProviderOptions{
				AutoEnabled:           true,
				ManualFallbackEnabled: true,
				TokenSkew:             10 * time.Second,
			},
		)
	)

	token, err := p.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}

	if token != "manual-access" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestAccessTokenFailsWhenNoSourceWorks(t *testing.T) {
	t.Parallel()

	var (
		p = NewProvider(
			fakeTokenRefresher{},
			&fakeStore{readErr: errors.New("not found")},
			fakeAutoSource{err: errors.New("auto")},
			fakeManualSource{err: errors.New("manual")},
			ProviderOptions{
				AutoEnabled:           true,
				ManualFallbackEnabled: true,
				TokenSkew:             10 * time.Second,
			},
		)
	)

	_, err := p.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected AccessToken() error")
	}
}

func TestClaimsReadsAccessTokenClaims(t *testing.T) {
	t.Parallel()

	const exp = int64(1777253137)
	p := NewProvider(
		fakeTokenRefresher{},
		&fakeStore{pair: domain.TokenPair{
			AccessToken: testJWT(`{"exp":1777253137,"isSubscribed":"1"}`),
			ExpiresAt:   time.Now().Add(15 * time.Minute),
		}},
		nil,
		nil,
		ProviderOptions{TokenSkew: 10 * time.Second},
	)

	claims, err := p.Claims(context.Background())
	if err != nil {
		t.Fatalf("Claims() error = %v", err)
	}
	if got := claims.ExpiresAt.Unix(); got != exp {
		t.Fatalf("claims exp = %d, want %d", got, exp)
	}
	if isSubscribed := claims.IsSubscribed(); !isSubscribed {
		t.Fatalf("claims isSubscribed = %t, want true", isSubscribed)
	}
}

func testJWT(payload string) string {
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature"
}
