package autobuild

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type tokenProviderStub struct {
	token  string
	claims domain.TokenClaims
	err    error
}

func (t tokenProviderStub) AccessToken(_ context.Context) (string, error) {
	if t.err != nil {
		return "", t.err
	}

	return t.token, nil
}

func (t tokenProviderStub) Refresh(_ context.Context) (domain.TokenPair, error) {
	return domain.TokenPair{AccessToken: t.token, ExpiresAt: time.Now().Add(10 * time.Minute)}, t.err
}

func (t tokenProviderStub) Claims(_ context.Context) (domain.TokenClaims, error) {
	if t.err != nil {
		return domain.TokenClaims{}, t.err
	}

	return t.claims, nil
}

type coachlessStub struct {
	mu              sync.Mutex
	getPatchesCalls int
	patches         []domain.PatchInfo
	treePlaycount   []domain.RuneTreePlaycount
	primaryRunes    *domain.RuneStatsByRow
	secondaryRunes  *domain.RuneStatsByRow
	shards          *domain.ShardStats
	keystoneCalls   []domain.KeystoneRequest
	treeCalls       []domain.SecondaryTreePlaycountRequest
	runeCalls       []domain.RuneStatsRequest
	shardCalls      []domain.ShardStatsRequest
	spellCalls      []domain.SummonerSpellStatsRequest
	itemCalls       []domain.ItemStatsRequest
	itemStats       []domain.ItemStat
	keystoneErr     error
	treeErr         error
	runeErr         error
	shardErr        error
	spellErr        error
	itemErr         error
}

func (c *coachlessStub) Refresh(_ context.Context, _ string) (domain.TokenPair, error) {
	return domain.TokenPair{}, errors.New("unused")
}

func (c *coachlessStub) GetPatches(_ context.Context, _ string) ([]domain.PatchInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getPatchesCalls++
	if c.patches != nil {
		return slices.Clone(c.patches), nil
	}

	return []domain.PatchInfo{{Label: "16.7", Major: 16, Patch: 7}}, nil
}

func (c *coachlessStub) GetKeystoneData(_ context.Context, _ string, req domain.KeystoneRequest) ([]domain.KeystoneStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.keystoneCalls = append(c.keystoneCalls, req)
	err := c.keystoneErr
	if err != nil {
		return nil, err
	}

	return []domain.KeystoneStat{{Rune: 8437, WPAOverall: 1.4, Occurrence: 1000}}, nil
}

func (c *coachlessStub) GetSecondaryTreePlaycount(_ context.Context, _ string, req domain.SecondaryTreePlaycountRequest) ([]domain.RuneTreePlaycount, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.treeCalls = append(c.treeCalls, req)
	if c.treeErr != nil {
		return nil, c.treeErr
	}
	if c.treePlaycount != nil {
		return slices.Clone(c.treePlaycount), nil
	}

	return []domain.RuneTreePlaycount{
		{Tree: domain.RuneStyleSorcery, Occurrence: 900},
		{Tree: domain.RuneStylePrecision, Occurrence: 500},
	}, nil
}

func (c *coachlessStub) GetRuneStatsForKeystoneAndTree(_ context.Context, _ string, req domain.RuneStatsRequest) (domain.RuneStatsByRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.runeCalls = append(c.runeCalls, req)
	if c.runeErr != nil {
		return domain.RuneStatsByRow{}, c.runeErr
	}

	switch req.TreeToLoad {
	case domain.RuneStyleResolve:
		if c.primaryRunes != nil {
			return *c.primaryRunes, nil
		}
		return domain.RuneStatsByRow{
			RowOnes:   []domain.RuneStat{{Rune: 8446, WPAOverall: 0.9, Occurrence: 1000}},
			RowTwos:   []domain.RuneStat{{Rune: 8444, WPAOverall: 0.8, Occurrence: 1000}},
			RowThrees: []domain.RuneStat{{Rune: 8451, WPAOverall: 0.7, Occurrence: 1000}},
		}, nil
	case domain.RuneStyleSorcery:
		if c.secondaryRunes != nil {
			return *c.secondaryRunes, nil
		}
		return domain.RuneStatsByRow{
			RowOnes:   []domain.RuneStat{{Rune: 8224, WPAOverall: 0.6, Occurrence: 1000}},
			RowTwos:   []domain.RuneStat{{Rune: 8233, WPAOverall: 0.9, Occurrence: 1000}},
			RowThrees: []domain.RuneStat{{Rune: 8237, WPAOverall: 0.5, Occurrence: 1000}},
		}, nil
	default:
		return domain.RuneStatsByRow{}, nil
	}
}

func (c *coachlessStub) GetShardStatsForKeystoneAndTree(_ context.Context, _ string, req domain.ShardStatsRequest) (domain.ShardStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.shardCalls = append(c.shardCalls, req)
	if c.shardErr != nil {
		return domain.ShardStats{}, c.shardErr
	}
	if c.shards != nil {
		return *c.shards, nil
	}

	return domain.ShardStats{
		Offense: []domain.RuneStat{{Rune: 5008, WPAOverall: 0.5, Occurrence: 1000}},
		Flex:    []domain.RuneStat{{Rune: 5008, WPAOverall: 0.4, Occurrence: 1000}},
		Defense: []domain.RuneStat{{Rune: 5002, WPAOverall: 0.3, Occurrence: 1000}},
	}, nil
}

func (c *coachlessStub) GetSummonerSpellStats(_ context.Context, _ string, req domain.SummonerSpellStatsRequest) ([]domain.SummonerSpellStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.spellCalls = append(c.spellCalls, req)
	err := c.spellErr
	if err != nil {
		return nil, err
	}

	return []domain.SummonerSpellStat{
		{SummonerSpell: 4, WPAOverall: 0.8, Occurrence: 500},
		{SummonerSpell: 14, WPAOverall: 0.7, Occurrence: 450},
	}, nil
}

func (c *coachlessStub) GetItemStats(_ context.Context, _ string, req domain.ItemStatsRequest) ([]domain.ItemStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.itemCalls = append(c.itemCalls, req)
	err := c.itemErr
	if err != nil {
		return nil, err
	}
	if c.itemStats != nil {
		return slices.Clone(c.itemStats), nil
	}

	return []domain.ItemStat{
		{ItemID: 1055, WPAOverall: 1.0, Occurrence: 900},
		{ItemID: 1036, WPAOverall: 0.5, Occurrence: 600},
	}, nil
}

type lcuStub struct {
	detectedSelection        domain.DetectedSelection
	detectErr                error
	detectCalls              int
	itemSetCalls             []domain.ApplyItemSetRequest
	runePageCalls            []domain.ApplyRunePageRequest
	runePageErr              error
	spellCalls               []domain.ApplySummonerSpellsRequest
	watchCalls               int
	watchEventsWithNoticesFn func(context.Context, chan<- domain.LCUEvent, chan<- domain.LCUWatchNotice) error
	watchErr                 error
}

func (l *lcuStub) DetectSelection(_ context.Context) (domain.DetectedSelection, error) {
	l.detectCalls++
	if l.detectErr != nil {
		return domain.DetectedSelection{}, l.detectErr
	}
	return l.detectedSelection, nil
}

func (l *lcuStub) ApplyItemSet(_ context.Context, req domain.ApplyItemSetRequest) error {
	l.itemSetCalls = append(l.itemSetCalls, req)
	return nil
}

func (l *lcuStub) ApplyRunePage(_ context.Context, req domain.ApplyRunePageRequest) error {
	l.runePageCalls = append(l.runePageCalls, req)
	return l.runePageErr
}

func (l *lcuStub) ApplySummonerSpells(_ context.Context, req domain.ApplySummonerSpellsRequest) error {
	l.spellCalls = append(l.spellCalls, req)
	return nil
}

func (l *lcuStub) WatchEventsWithNotices(ctx context.Context, out chan<- domain.LCUEvent, notices chan<- domain.LCUWatchNotice) error {
	l.watchCalls++
	if l.watchEventsWithNoticesFn != nil {
		return l.watchEventsWithNoticesFn(ctx, out, notices)
	}
	return l.watchErr
}
