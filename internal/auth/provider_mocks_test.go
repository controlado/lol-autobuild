package auth

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type fakeStore struct {
	pair       domain.TokenPair
	readErr    error
	writeErr   error
	clearErr   error
	writeCalls int
	clearCalls int
}

func (s *fakeStore) ReadTokens(_ context.Context) (domain.TokenPair, error) {
	if s.readErr != nil {
		return domain.TokenPair{}, s.readErr
	}

	return s.pair, nil
}

func (s *fakeStore) WriteTokens(_ context.Context, pair domain.TokenPair) error {
	s.writeCalls++
	if s.writeErr != nil {
		return s.writeErr
	}

	s.pair = pair
	return nil
}

func (s *fakeStore) ClearTokens(_ context.Context) error {
	s.clearCalls++
	if s.clearErr != nil {
		return s.clearErr
	}

	s.pair = domain.TokenPair{}
	return nil
}

type fakeTokenRefresher struct {
	refreshed domain.TokenPair
	err       error
}

func (f fakeTokenRefresher) Refresh(_ context.Context, _ string) (domain.TokenPair, error) {
	if f.err != nil {
		return domain.TokenPair{}, f.err
	}

	return f.refreshed, nil
}

type fakeManualSource struct {
	pair domain.TokenPair
	err  error
}

func (m fakeManualSource) Acquire(_ context.Context) (domain.TokenPair, error) {
	if m.err != nil {
		return domain.TokenPair{}, m.err
	}

	return m.pair, nil
}

type fakeAutoSource struct {
	pair domain.TokenPair
	err  error
}

func (a fakeAutoSource) Acquire(_ context.Context) (domain.TokenPair, error) {
	if a.err != nil {
		return domain.TokenPair{}, a.err
	}

	return a.pair, nil
}
