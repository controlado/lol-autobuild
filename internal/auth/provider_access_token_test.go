package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func TestAccessTokenRefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	var (
		store = &fakeStore{
			pair: ports.TokenPair{
				AccessToken:  "expired",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(-1 * time.Minute),
			},
		}
		p = NewProvider(
			fakeCoachless{
				refreshed: ports.TokenPair{
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

func TestAccessTokenFallsBackToManual(t *testing.T) {
	t.Parallel()

	var (
		p = NewProvider(
			fakeCoachless{},
			&fakeStore{readErr: errors.New("not found")},
			fakeAutoSource{err: errors.New("auto fail")},
			fakeManualSource{
				pair: ports.TokenPair{
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
			fakeCoachless{},
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
