package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type (
	TokenRefresher interface {
		Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error)
	}
	SecretStore interface {
		ReadTokens(ctx context.Context) (domain.TokenPair, error)
		WriteTokens(ctx context.Context, pair domain.TokenPair) error
	}
)

type ProviderOptions struct {
	AutoEnabled           bool
	ManualFallbackEnabled bool
	TokenSkew             time.Duration
}

type Provider struct {
	tokenRefresher TokenRefresher
	store          SecretStore
	auto           AutoSource
	manual         ManualSource
	opts           ProviderOptions
}

func NewProvider(tokenRefresher TokenRefresher, store SecretStore, auto AutoSource, manual ManualSource, opts ProviderOptions) *Provider {
	return &Provider{
		tokenRefresher: tokenRefresher,
		store:          store,
		auto:           auto,
		manual:         manual,
		opts:           opts,
	}
}

func (p *Provider) AccessToken(ctx context.Context) (string, error) {
	pair, err := p.store.ReadTokens(ctx)
	if err == nil {
		pair = ensureExpiry(pair)
		if pair.AccessToken != "" && isTokenValid(pair.ExpiresAt, p.opts.TokenSkew) {
			return pair.AccessToken, nil
		}

		if pair.RefreshToken != "" {
			refreshed, refreshErr := p.tokenRefresher.Refresh(ctx, pair.RefreshToken)
			if refreshErr == nil {
				refreshed, refreshErr = validateRefreshedTokenPair(refreshed)
			}
			if refreshErr == nil {
				refreshed = ensureExpiry(refreshed)
				if err := p.store.WriteTokens(ctx, refreshed); err != nil {
					return "", fmt.Errorf("persist refreshed tokens: %w", err)
				}

				return refreshed.AccessToken, nil
			}
		}
	}

	if p.opts.AutoEnabled && p.auto != nil {
		autoPair, autoErr := p.auto.Acquire(ctx)
		if autoErr == nil {
			autoPair = ensureExpiry(autoPair)
			if err := p.store.WriteTokens(ctx, autoPair); err != nil {
				return "", fmt.Errorf("persist auto tokens: %w", err)
			}

			return autoPair.AccessToken, nil
		}
	}

	if p.opts.ManualFallbackEnabled && p.manual != nil {
		manualPair, manualErr := p.manual.Acquire(ctx)
		if manualErr == nil {
			manualPair = ensureExpiry(manualPair)
			if err := p.store.WriteTokens(ctx, manualPair); err != nil {
				return "", fmt.Errorf("persist manual tokens: %w", err)
			}

			return manualPair.AccessToken, nil
		}
	}

	return "", ErrAccessTokenUnavailable
}

func (p *Provider) Refresh(ctx context.Context) (domain.TokenPair, error) {
	pair, err := p.store.ReadTokens(ctx)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("read tokens: %w", err)
	}

	if strings.TrimSpace(pair.RefreshToken) == "" {
		return domain.TokenPair{}, errors.New("no refresh token available")
	}

	refreshed, err := p.tokenRefresher.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("refresh token: %w", err)
	}
	refreshed, err = validateRefreshedTokenPair(refreshed)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("refresh token: %w", err)
	}
	refreshed = ensureExpiry(refreshed)

	if err := p.store.WriteTokens(ctx, refreshed); err != nil {
		return domain.TokenPair{}, fmt.Errorf("persist refreshed tokens: %w", err)
	}

	return refreshed, nil
}

func (p *Provider) Claims(ctx context.Context) (domain.TokenClaims, error) {
	accessToken, err := p.AccessToken(ctx)
	if err != nil {
		return domain.TokenClaims{}, err
	}

	return claimsFromJWT(accessToken)
}

func isTokenValid(exp time.Time, skew time.Duration) bool {
	if exp.IsZero() {
		return false
	}

	return time.Now().Add(skew).Before(exp)
}

func ensureExpiry(pair domain.TokenPair) domain.TokenPair {
	if !pair.ExpiresAt.IsZero() {
		return pair
	}

	exp, err := expFromJWT(pair.AccessToken)
	if err != nil {
		return pair
	}

	pair.ExpiresAt = exp
	return pair
}

func expFromJWT(token string) (time.Time, error) {
	claims, err := claimsFromJWT(token)
	if err != nil {
		return time.Time{}, err
	}

	if claims.ExpiresAt.IsZero() {
		return time.Time{}, errors.New("exp claim missing")
	}

	return claims.ExpiresAt, nil
}

func validateRefreshedTokenPair(pair domain.TokenPair) (domain.TokenPair, error) {
	pair.AccessToken = strings.TrimSpace(pair.AccessToken)
	pair.RefreshToken = strings.TrimSpace(pair.RefreshToken)
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		return domain.TokenPair{}, errors.New("refreshed token pair missing access or refresh token")
	}

	return pair, nil
}

func claimsFromJWT(token string) (domain.TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return domain.TokenClaims{}, errors.New("invalid jwt format")
	}

	payload := parts[1]
	payload = strings.ReplaceAll(payload, "-", "+")
	payload = strings.ReplaceAll(payload, "_", "/")
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return domain.TokenClaims{}, fmt.Errorf("decode payload: %w", err)
	}

	var body jwtClaims
	if err := json.Unmarshal(decoded, &body); err != nil {
		return domain.TokenClaims{}, fmt.Errorf("parse payload: %w", err)
	}

	return domain.TokenClaims{
		ExpiresAt:  time.Unix(body.Exp, 0).UTC(),
		Subscribed: body.IsSubscribedRaw == "1",
	}, nil
}

type jwtClaims struct {
	Exp             int64  `json:"exp"`
	IsSubscribedRaw string `json:"isSubscribed"`
}
