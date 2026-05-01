package lolautobuild

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type tokenProviderStub struct {
	token  string
	claims ports.TokenClaims
	err    error
}

func (t tokenProviderStub) AccessToken(_ context.Context) (string, error) {
	if t.err != nil {
		return "", t.err
	}

	return t.token, nil
}

func (t tokenProviderStub) Refresh(_ context.Context) (ports.TokenPair, error) {
	return ports.TokenPair{AccessToken: t.token, ExpiresAt: time.Now().Add(10 * time.Minute)}, t.err
}

func (t tokenProviderStub) Claims(_ context.Context) (ports.TokenClaims, error) {
	if t.err != nil {
		return ports.TokenClaims{}, t.err
	}

	return t.claims, nil
}

type coachlessStub struct {
	mu              sync.Mutex
	getPatchesCalls int
	patches         []ports.PatchInfo
	keystoneCalls   []ports.KeystoneRequest
	spellCalls      []ports.SummonerSpellStatsRequest
	itemCalls       []ports.ItemStatsRequest
	itemStats       []ports.ItemStat
	keystoneErr     error
	spellErr        error
	itemErr         error
}

func (c *coachlessStub) Refresh(_ context.Context, _ string) (ports.TokenPair, error) {
	return ports.TokenPair{}, errors.New("unused")
}

func (c *coachlessStub) GetPatches(_ context.Context, _ string) ([]ports.PatchInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getPatchesCalls++
	if c.patches != nil {
		return append([]ports.PatchInfo{}, c.patches...), nil
	}

	return []ports.PatchInfo{{Label: "16.7", Major: 16, Patch: 7, MatchCount: 1}}, nil
}

func (c *coachlessStub) GetKeystoneData(_ context.Context, _ string, req ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.keystoneCalls = append(c.keystoneCalls, req)
	err := c.keystoneErr
	if err != nil {
		return nil, err
	}

	return []ports.KeystoneStat{{Rune: 8437, WPAOverall: 1.4, Occurrence: 1000}}, nil
}

func (c *coachlessStub) GetSummonerSpellStats(_ context.Context, _ string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.spellCalls = append(c.spellCalls, req)
	err := c.spellErr
	if err != nil {
		return nil, err
	}

	return []ports.SummonerSpellStat{
		{SummonerSpell: 4, WPAOverall: 0.8, Occurrence: 500},
		{SummonerSpell: 14, WPAOverall: 0.7, Occurrence: 450},
	}, nil
}

func (c *coachlessStub) GetItemStats(_ context.Context, _ string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.itemCalls = append(c.itemCalls, req)
	err := c.itemErr
	if err != nil {
		return nil, err
	}
	if c.itemStats != nil {
		return append([]ports.ItemStat{}, c.itemStats...), nil
	}

	return []ports.ItemStat{
		{ItemID: 1055, WPAOverall: 1.0, Occurrence: 900},
		{ItemID: 1036, WPAOverall: 0.5, Occurrence: 600},
	}, nil
}

type lcuStub struct {
	detectedSelection        ports.DetectedSelection
	detectErr                error
	detectCalls              int
	itemSetCalls             []ports.ApplyItemSetRequest
	runePageCalls            []ports.ApplyRunePageRequest
	spellCalls               []ports.ApplySummonerSpellsRequest
	watchCalls               int
	watchEventsWithNoticesFn func(context.Context, chan<- ports.LCUEvent, chan<- ports.LCUWatchNotice) error
	watchErr                 error
}

func (l *lcuStub) DetectSelection(_ context.Context) (ports.DetectedSelection, error) {
	l.detectCalls++
	if l.detectErr != nil {
		return ports.DetectedSelection{}, l.detectErr
	}
	return l.detectedSelection, nil
}

func (l *lcuStub) ApplyItemSet(_ context.Context, req ports.ApplyItemSetRequest) error {
	l.itemSetCalls = append(l.itemSetCalls, req)
	return nil
}

func (l *lcuStub) ApplyRunePage(_ context.Context, req ports.ApplyRunePageRequest) error {
	l.runePageCalls = append(l.runePageCalls, req)
	return nil
}

func (l *lcuStub) ApplySummonerSpells(_ context.Context, req ports.ApplySummonerSpellsRequest) error {
	l.spellCalls = append(l.spellCalls, req)
	return nil
}

func (l *lcuStub) WatchEventsWithNotices(ctx context.Context, out chan<- ports.LCUEvent, notices chan<- ports.LCUWatchNotice) error {
	l.watchCalls++
	if l.watchEventsWithNoticesFn != nil {
		return l.watchEventsWithNoticesFn(ctx, out, notices)
	}
	return l.watchErr
}
