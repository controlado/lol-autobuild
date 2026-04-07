package lolautobuild

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/recommend"
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

type coachlessStub struct{}

func (coachlessStub) Refresh(ctx context.Context, refreshToken string) (ports.TokenPair, error) {
	_ = ctx
	_ = refreshToken
	return ports.TokenPair{}, errors.New("unused")
}

func (coachlessStub) GetPatches(ctx context.Context, accessToken string) ([]ports.PatchInfo, error) {
	_ = ctx
	_ = accessToken
	return []ports.PatchInfo{{Label: "16.7", Major: 16, Patch: 7, MatchCount: 1}}, nil
}

func (coachlessStub) GetKeystoneData(ctx context.Context, accessToken string, req ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return []ports.KeystoneStat{{Rune: 8437, WPAOverall: 1.4, Occurrence: 1000}}, nil
}

func (coachlessStub) GetSummonerSpellStats(ctx context.Context, accessToken string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return []ports.SummonerSpellStat{
		{SummonerSpell: 4, WPAOverall: 0.8, Occurrence: 500},
		{SummonerSpell: 14, WPAOverall: 0.7, Occurrence: 450},
	}, nil
}

func (coachlessStub) GetItemStats(ctx context.Context, accessToken string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	_ = ctx
	_ = accessToken
	_ = req
	return []ports.ItemStat{
		{ItemID: 1055, WPAOverall: 1.0, Occurrence: 900},
		{ItemID: 1036, WPAOverall: 0.5, Occurrence: 600},
	}, nil
}

type lcuStub struct {
	itemCalls  int
	runeCalls  int
	spellCalls int
}

func (l *lcuStub) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	_ = req
	l.itemCalls++
	return nil
}

func (l *lcuStub) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	_ = req
	l.runeCalls++
	return nil
}

func (l *lcuStub) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	_ = req
	l.spellCalls++
	return nil
}

func TestSyncDryRunDoesNotCallLCU(t *testing.T) {
	t.Parallel()

	lcu := &lcuStub{}
	svc, err := NewService(ServiceDeps{
		Coachless:   coachlessStub{},
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ChampionID:  240,
		Role:        "top",
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.ItemSetApplied || got.RunePageApplied || got.SpellsApplied {
		t.Fatalf("expected no applied flags in dry-run, got %#v", got)
	}

	if lcu.itemCalls+lcu.runeCalls+lcu.spellCalls != 0 {
		t.Fatalf("expected no LCU calls in dry-run, got items=%d runes=%d spells=%d", lcu.itemCalls, lcu.runeCalls, lcu.spellCalls)
	}
}

func TestSyncRespectsApplyFlags(t *testing.T) {
	t.Parallel()

	lcu := &lcuStub{}
	svc, err := NewService(ServiceDeps{
		Coachless:   coachlessStub{},
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ChampionID:  240,
		Role:        "top",
		ApplyItems:  true,
		ApplyRunes:  false,
		ApplySpells: false,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if !got.ItemSetApplied || got.RunePageApplied || got.SpellsApplied {
		t.Fatalf("unexpected apply result: %#v", got)
	}

	if lcu.itemCalls != 1 || lcu.runeCalls != 0 || lcu.spellCalls != 0 {
		t.Fatalf("unexpected LCU calls: items=%d runes=%d spells=%d", lcu.itemCalls, lcu.runeCalls, lcu.spellCalls)
	}
}
