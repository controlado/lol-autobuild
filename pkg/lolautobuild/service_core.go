package lolautobuild

import (
	"context"
	"errors"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type RecommendationPolicy struct {
	MinOccurrence int
	TopItems      int
	TopSpells     int
}

type ServiceDeps struct {
	Coachless   ports.CoachlessClient
	Tokens      ports.TokenProvider
	LCU         ports.LCUClient
	Recommender ports.RecommendationEngine
	Policy      RecommendationPolicy
}

type syncService struct {
	deps ServiceDeps
}

func NewService(deps ServiceDeps) (Service, error) {
	if deps.Coachless == nil {
		return nil, errors.New("coachless client is required")
	}
	if deps.Tokens == nil {
		return nil, errors.New("token provider is required")
	}
	if deps.LCU == nil {
		return nil, errors.New("lcu client is required")
	}
	if deps.Recommender == nil {
		return nil, errors.New("recommendation engine is required")
	}
	if deps.Policy.MinOccurrence < 0 {
		return nil, errors.New("policy.min_occurrence must be >= 0")
	}
	if deps.Policy.TopItems < 0 {
		return nil, errors.New("policy.top_items must be >= 0")
	}
	if deps.Policy.TopSpells <= 0 {
		deps.Policy.TopSpells = 2
	}

	return &syncService{deps: deps}, nil
}

func (s *syncService) EnsureCoachlessAuth(ctx context.Context) error {
	if _, err := s.deps.Tokens.AccessToken(ctx); err != nil {
		return err
	}

	return nil
}
