package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type ProviderOptions struct {
	AutoEnabled           bool
	ManualFallbackEnabled bool
	TokenSkew             time.Duration
}

type Provider struct {
	coachless ports.CoachlessClient
	store     ports.SecretStore
	auto      AutoSource
	manual    ManualSource
	opts      ProviderOptions
}

func NewProvider(coachless ports.CoachlessClient, store ports.SecretStore, auto AutoSource, manual ManualSource, opts ProviderOptions) *Provider {
	return &Provider{
		coachless: coachless,
		store:     store,
		auto:      auto,
		manual:    manual,
		opts:      opts,
	}
}

func (p *Provider) AccessToken(ctx context.Context) (string, error) {
	var causes []error

	pair, err := p.store.ReadTokens(ctx)
	if err == nil {
		pair = ensureExpiry(pair)
		if pair.AccessToken != "" && isTokenValid(pair.ExpiresAt, p.opts.TokenSkew) {
			return pair.AccessToken, nil
		}

		if pair.RefreshToken != "" {
			refreshed, refreshErr := p.coachless.Refresh(ctx, pair.RefreshToken)
			if refreshErr == nil {
				refreshed = ensureExpiry(refreshed)
				if err := p.store.WriteTokens(ctx, refreshed); err != nil {
					return "", fmt.Errorf("persist refreshed tokens: %w", err)
				}

				return refreshed.AccessToken, nil
			}

			causes = append(causes, fmt.Errorf("refresh token: %w", refreshErr))
		}
	} else {
		causes = append(causes, fmt.Errorf("read tokens: %w", err))
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

		causes = append(causes, fmt.Errorf("auto acquire tokens: %w", autoErr))
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

		causes = append(causes, fmt.Errorf("manual acquire tokens: %w", manualErr))
	}

	baseErr := errors.New("unable to acquire valid access token")
	if len(causes) == 0 {
		return "", baseErr
	}

	return "", errors.Join(append([]error{baseErr}, causes...)...)
}

func (p *Provider) Refresh(ctx context.Context) (ports.TokenPair, error) {
	pair, err := p.store.ReadTokens(ctx)
	if err != nil {
		return ports.TokenPair{}, fmt.Errorf("read tokens: %w", err)
	}

	if strings.TrimSpace(pair.RefreshToken) == "" {
		return ports.TokenPair{}, errors.New("no refresh token available")
	}

	refreshed, err := p.coachless.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		return ports.TokenPair{}, fmt.Errorf("refresh token: %w", err)
	}

	refreshed = ensureExpiry(refreshed)
	if err := p.store.WriteTokens(ctx, refreshed); err != nil {
		return ports.TokenPair{}, fmt.Errorf("persist refreshed tokens: %w", err)
	}

	return refreshed, nil
}

func isTokenValid(exp time.Time, skew time.Duration) bool {
	if exp.IsZero() {
		return false
	}

	return time.Now().Add(skew).Before(exp)
}

func ensureExpiry(pair ports.TokenPair) ports.TokenPair {
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
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, errors.New("invalid jwt format")
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
		return time.Time{}, fmt.Errorf("decode payload: %w", err)
	}

	var body struct {
		Exp int64 `json:"exp"`
	}

	if err := json.Unmarshal(decoded, &body); err != nil {
		return time.Time{}, fmt.Errorf("parse payload: %w", err)
	}

	if body.Exp == 0 {
		return time.Time{}, errors.New("exp claim missing")
	}

	return time.Unix(body.Exp, 0).UTC(), nil
}
