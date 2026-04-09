package lolautobuild

import (
	"context"
	"errors"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type tokenProviderStub struct {
	token string
	err   error
}

func (t tokenProviderStub) AccessToken(ctx context.Context) (string, error) {
	_ = ctx
	if t.err != nil {
		return "", t.err
	}

	return t.token, nil
}

func (t tokenProviderStub) Refresh(ctx context.Context) (ports.TokenPair, error) {
	_ = ctx
	return ports.TokenPair{AccessToken: t.token, ExpiresAt: time.Now().Add(10 * time.Minute)}, t.err
}

type coachlessStub struct {
	getPatchesCalls int
	keystoneCalls   []ports.KeystoneRequest
	spellCalls      []ports.SummonerSpellStatsRequest
	itemCalls       []ports.ItemStatsRequest
	keystoneErr     error
	spellErr        error
	itemErr         error
}

func (c *coachlessStub) Refresh(ctx context.Context, refreshToken string) (ports.TokenPair, error) {
	_ = ctx
	_ = refreshToken
	return ports.TokenPair{}, errors.New("unused")
}

func (c *coachlessStub) GetPatches(ctx context.Context, accessToken string) ([]ports.PatchInfo, error) {
	_ = ctx
	_ = accessToken
	c.getPatchesCalls++
	return []ports.PatchInfo{{Label: "16.7", Major: 16, Patch: 7, MatchCount: 1}}, nil
}

func (c *coachlessStub) GetKeystoneData(ctx context.Context, accessToken string, req ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	_ = ctx
	_ = accessToken
	c.keystoneCalls = append(c.keystoneCalls, req)
	if c.keystoneErr != nil {
		return nil, c.keystoneErr
	}
	return []ports.KeystoneStat{{Rune: 8437, WPAOverall: 1.4, Occurrence: 1000}}, nil
}

func (c *coachlessStub) GetSummonerSpellStats(ctx context.Context, accessToken string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	_ = ctx
	_ = accessToken
	c.spellCalls = append(c.spellCalls, req)
	if c.spellErr != nil {
		return nil, c.spellErr
	}
	return []ports.SummonerSpellStat{
		{SummonerSpell: 4, WPAOverall: 0.8, Occurrence: 500},
		{SummonerSpell: 14, WPAOverall: 0.7, Occurrence: 450},
	}, nil
}

func (c *coachlessStub) GetItemStats(ctx context.Context, accessToken string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	_ = ctx
	_ = accessToken
	c.itemCalls = append(c.itemCalls, req)
	if c.itemErr != nil {
		return nil, c.itemErr
	}
	return []ports.ItemStat{
		{ItemID: 1055, WPAOverall: 1.0, Occurrence: 900},
		{ItemID: 1036, WPAOverall: 0.5, Occurrence: 600},
	}, nil
}

type lcuStub struct {
	detectedSelection ports.DetectedSelection
	detectErr         error
	detectCalls       int
	itemSetCalls      []ports.ApplyItemSetRequest
	runePageCalls     []ports.ApplyRunePageRequest
	spellCalls        []ports.ApplySummonerSpellsRequest
	watchEventsErr    error
}

func (l *lcuStub) DetectSelection(ctx context.Context) (ports.DetectedSelection, error) {
	_ = ctx
	l.detectCalls++
	if l.detectErr != nil {
		return ports.DetectedSelection{}, l.detectErr
	}
	return l.detectedSelection, nil
}

func (l *lcuStub) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	l.itemSetCalls = append(l.itemSetCalls, req)
	return nil
}

func (l *lcuStub) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	l.runePageCalls = append(l.runePageCalls, req)
	return nil
}

func (l *lcuStub) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	l.spellCalls = append(l.spellCalls, req)
	return nil
}

func (l *lcuStub) WatchEvents(ctx context.Context, out chan<- ports.LCUEvent) error {
	_ = ctx
	_ = out
	return l.watchEventsErr
}
