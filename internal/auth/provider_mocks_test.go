package auth

import (
	"context"
	"errors"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type fakeStore struct {
	pair     ports.TokenPair
	readErr  error
	writeErr error
}

func (s *fakeStore) ReadTokens(ctx context.Context) (ports.TokenPair, error) {
	_ = ctx
	if s.readErr != nil {
		return ports.TokenPair{}, s.readErr
	}

	return s.pair, nil
}

func (s *fakeStore) WriteTokens(ctx context.Context, pair ports.TokenPair) error {
	_ = ctx
	if s.writeErr != nil {
		return s.writeErr
	}

	s.pair = pair
	return nil
}

func (s *fakeStore) ClearTokens(ctx context.Context) error {
	_ = ctx
	s.pair = ports.TokenPair{}
	return nil
}

type fakeCoachless struct {
	refreshed ports.TokenPair
	err       error
}

func (f fakeCoachless) Refresh(ctx context.Context, refreshToken string) (ports.TokenPair, error) {
	_ = ctx
	_ = refreshToken
	if f.err != nil {
		return ports.TokenPair{}, f.err
	}

	return f.refreshed, nil
}

func (fakeCoachless) GetPatches(ctx context.Context, accessToken string) ([]ports.PatchInfo, error) {
	_ = ctx
	_ = accessToken
	return nil, errors.New("unused")
}

func (fakeCoachless) GetKeystoneData(ctx context.Context, accessToken string, req ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return nil, errors.New("unused")
}

func (fakeCoachless) GetSummonerSpellStats(ctx context.Context, accessToken string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return nil, errors.New("unused")
}

func (fakeCoachless) GetItemStats(ctx context.Context, accessToken string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return nil, errors.New("unused")
}

type fakeManualSource struct {
	pair ports.TokenPair
	err  error
}

func (m fakeManualSource) Acquire(ctx context.Context) (ports.TokenPair, error) {
	_ = ctx
	if m.err != nil {
		return ports.TokenPair{}, m.err
	}

	return m.pair, nil
}

type fakeAutoSource struct {
	pair ports.TokenPair
	err  error
}

func (a fakeAutoSource) Acquire(ctx context.Context) (ports.TokenPair, error) {
	_ = ctx
	if a.err != nil {
		return ports.TokenPair{}, a.err
	}

	return a.pair, nil
}
