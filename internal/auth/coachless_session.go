package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

const (
	CoachlessAuthBundleType    = "lol-autobuild.coachless.auth"
	CoachlessAuthBundleVersion = 1
)

type CoachlessSessionStatus string

const (
	CoachlessSessionStatusMissing CoachlessSessionStatus = "missing"
	CoachlessSessionStatusStored  CoachlessSessionStatus = "stored"
	CoachlessSessionStatusExpired CoachlessSessionStatus = "expired"
	CoachlessSessionStatusError   CoachlessSessionStatus = "error"
)

type CoachlessPlan string

const (
	CoachlessPlanUnknown CoachlessPlan = "unknown"
	CoachlessPlanFree    CoachlessPlan = "free"
	CoachlessPlanPremium CoachlessPlan = "premium"
)

type CoachlessSessionOptions struct {
	TokenSkew time.Duration
}

type CoachlessSessionState struct {
	Status    CoachlessSessionStatus
	Plan      CoachlessPlan
	ExpiresAt *time.Time
	Message   string
}

type CoachlessAuthBundle struct {
	Type                 string     `json:"type"`
	Version              int        `json:"version"`
	AccessToken          string     `json:"access_token"`
	RefreshToken         string     `json:"refresh_token"`
	AccessTokenExpiresAt *time.Time `json:"access_token_expires_at,omitempty"`
}

type CoachlessSession struct {
	tokenRefresher TokenRefresher
	store          ClearableTokenStore
	auto           AutoSource
	opts           CoachlessSessionOptions
}

func NewCoachlessSession(tokenRefresher TokenRefresher, store ClearableTokenStore, auto AutoSource, opts CoachlessSessionOptions) *CoachlessSession {
	return &CoachlessSession{
		tokenRefresher: tokenRefresher,
		store:          store,
		auto:           auto,
		opts:           opts,
	}
}

func (s *CoachlessSession) Status(ctx context.Context) CoachlessSessionState {
	pair, err := s.store.ReadTokens(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrTokensNotFound) {
			return CoachlessSessionState{Status: CoachlessSessionStatusMissing, Plan: CoachlessPlanUnknown}
		}

		return CoachlessSessionState{
			Status:  CoachlessSessionStatusError,
			Plan:    CoachlessPlanUnknown,
			Message: "Coachless authentication status is unavailable.",
		}
	}

	return statusFromTokenPair(pair, s.opts.TokenSkew)
}

func (s *CoachlessSession) Login(ctx context.Context) error {
	if s.auto == nil {
		return ErrAccessTokenUnavailable
	}

	pair, err := s.auto.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire coachless auth: %w", err)
	}

	pair, err = validateRefreshedTokenPair(pair)
	if err != nil {
		return err
	}
	pair = ensureExpiry(pair)

	if err := s.store.WriteTokens(ctx, pair); err != nil {
		return fmt.Errorf("persist coachless auth: %w", err)
	}

	return nil
}

func (s *CoachlessSession) Logout(ctx context.Context) error {
	if err := s.store.ClearTokens(ctx); err != nil {
		return fmt.Errorf("clear coachless auth: %w", err)
	}

	return nil
}

func (s *CoachlessSession) Export(ctx context.Context) (string, error) {
	pair, err := s.store.ReadTokens(ctx)
	if err != nil {
		return "", fmt.Errorf("read coachless auth: %w", err)
	}

	pair.AccessToken = strings.TrimSpace(pair.AccessToken)
	pair.RefreshToken = strings.TrimSpace(pair.RefreshToken)
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		return "", errors.New("stored coachless auth is missing access or refresh token")
	}
	pair = ensureExpiry(pair)

	bundle := CoachlessAuthBundle{
		Type:         CoachlessAuthBundleType,
		Version:      CoachlessAuthBundleVersion,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}
	if !pair.ExpiresAt.IsZero() {
		expiresAt := pair.ExpiresAt.UTC()
		bundle.AccessTokenExpiresAt = &expiresAt
	}

	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode coachless auth bundle: %w", err)
	}

	return string(raw), nil
}

func (s *CoachlessSession) Import(ctx context.Context, raw string) error {
	var bundle CoachlessAuthBundle
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return fmt.Errorf("decode coachless auth bundle: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("coachless auth bundle must contain one json object")
	}

	if bundle.Type != CoachlessAuthBundleType {
		return errors.New("coachless auth bundle type is invalid")
	}
	if bundle.Version != CoachlessAuthBundleVersion {
		return errors.New("coachless auth bundle version is unsupported")
	}

	refreshToken := strings.TrimSpace(bundle.RefreshToken)
	if refreshToken == "" {
		return errors.New("coachless auth bundle refresh token is required")
	}
	if s.tokenRefresher == nil {
		return errors.New("coachless auth refresh is unavailable")
	}

	refreshed, err := s.tokenRefresher.Refresh(ctx, refreshToken)
	if err != nil {
		return fmt.Errorf("refresh imported coachless auth: %w", err)
	}
	refreshed, err = validateRefreshedTokenPair(refreshed)
	if err != nil {
		return fmt.Errorf("refresh imported coachless auth: %w", err)
	}
	refreshed = ensureExpiry(refreshed)

	if err := s.store.WriteTokens(ctx, refreshed); err != nil {
		return fmt.Errorf("persist imported coachless auth: %w", err)
	}

	return nil
}

func statusFromTokenPair(pair domain.TokenPair, skew time.Duration) CoachlessSessionState {
	pair.AccessToken = strings.TrimSpace(pair.AccessToken)
	pair.RefreshToken = strings.TrimSpace(pair.RefreshToken)
	if pair.AccessToken == "" && pair.RefreshToken == "" {
		return CoachlessSessionState{Status: CoachlessSessionStatusMissing, Plan: CoachlessPlanUnknown}
	}

	var (
		claims    domain.TokenClaims
		hasClaims bool
	)
	if pair.AccessToken != "" {
		parsed, err := parseClaims(pair.AccessToken)
		if err == nil {
			claims = parsed
			hasClaims = true
			if pair.ExpiresAt.IsZero() {
				pair.ExpiresAt = parsed.ExpiresAt
			}
		} else if pair.RefreshToken == "" {
			return CoachlessSessionState{
				Status:  CoachlessSessionStatusError,
				Plan:    CoachlessPlanUnknown,
				Message: "Stored Coachless authentication is invalid.",
			}
		}
	}

	status := CoachlessSessionStatusStored
	if !pair.ExpiresAt.IsZero() && !isTokenValid(pair.ExpiresAt, skew) {
		status = CoachlessSessionStatusExpired
	}

	sessionStatus := CoachlessSessionState{
		Status: status,
		Plan:   CoachlessPlanUnknown,
	}
	if !pair.ExpiresAt.IsZero() {
		expiresAt := pair.ExpiresAt.UTC()
		sessionStatus.ExpiresAt = &expiresAt
	}
	if hasClaims {
		if claims.IsSubscribed() {
			sessionStatus.Plan = CoachlessPlanPremium
		} else {
			sessionStatus.Plan = CoachlessPlanFree
		}
	}

	return sessionStatus
}
