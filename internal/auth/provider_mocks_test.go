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

func (s *fakeStore) ReadTokens(_ context.Context) (ports.TokenPair, error) {
	if s.readErr != nil {
		return ports.TokenPair{}, s.readErr
	}

	return s.pair, nil
}

func (s *fakeStore) WriteTokens(_ context.Context, pair ports.TokenPair) error {
	if s.writeErr != nil {
		return s.writeErr
	}

	s.pair = pair
	return nil
}

func (s *fakeStore) ClearTokens(_ context.Context) error {
	s.pair = ports.TokenPair{}
	return nil
}

type fakeCoachless struct {
	refreshed ports.TokenPair
	err       error
}

func (f fakeCoachless) Refresh(_ context.Context, _ string) (ports.TokenPair, error) {
	if f.err != nil {
		return ports.TokenPair{}, f.err
	}

	return f.refreshed, nil
}

func (fakeCoachless) GetPatches(_ context.Context, _ string) ([]ports.PatchInfo, error) {
	return nil, errors.New("unused")
}

func (fakeCoachless) GetKeystoneData(_ context.Context, _ string, _ ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	return nil, errors.New("unused")
}

func (fakeCoachless) GetSecondaryTreePlaycount(_ context.Context, _ string, _ ports.SecondaryTreePlaycountRequest) ([]ports.RuneTreePlaycount, error) {
	return nil, errors.New("unused")
}

func (fakeCoachless) GetRuneStatsForKeystoneAndTree(_ context.Context, _ string, _ ports.RuneStatsRequest) (ports.RuneStatsByRow, error) {
	return ports.RuneStatsByRow{}, errors.New("unused")
}

func (fakeCoachless) GetShardStatsForKeystoneAndTree(_ context.Context, _ string, _ ports.ShardStatsRequest) (ports.ShardStats, error) {
	return ports.ShardStats{}, errors.New("unused")
}

func (fakeCoachless) GetSummonerSpellStats(_ context.Context, _ string, _ ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	return nil, errors.New("unused")
}

func (fakeCoachless) GetItemStats(_ context.Context, _ string, _ ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	return nil, errors.New("unused")
}

type fakeManualSource struct {
	pair ports.TokenPair
	err  error
}

func (m fakeManualSource) Acquire(_ context.Context) (ports.TokenPair, error) {
	if m.err != nil {
		return ports.TokenPair{}, m.err
	}

	return m.pair, nil
}

type fakeAutoSource struct {
	pair ports.TokenPair
	err  error
}

func (a fakeAutoSource) Acquire(_ context.Context) (ports.TokenPair, error) {
	if a.err != nil {
		return ports.TokenPair{}, a.err
	}

	return a.pair, nil
}
