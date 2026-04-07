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

type coachlessStub struct {
	getPatchesCalls int
	keystoneCalls   []ports.KeystoneRequest
	spellCalls      []ports.SummonerSpellStatsRequest
	itemCalls       []ports.ItemStatsRequest
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
	return []ports.KeystoneStat{{Rune: 8437, WPAOverall: 1.4, Occurrence: 1000}}, nil
}

func (c *coachlessStub) GetSummonerSpellStats(ctx context.Context, accessToken string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	_ = ctx
	_ = accessToken
	c.spellCalls = append(c.spellCalls, req)
	return []ports.SummonerSpellStat{
		{SummonerSpell: 4, WPAOverall: 0.8, Occurrence: 500},
		{SummonerSpell: 14, WPAOverall: 0.7, Occurrence: 450},
	}, nil
}

func (c *coachlessStub) GetItemStats(ctx context.Context, accessToken string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	_ = ctx
	_ = accessToken
	c.itemCalls = append(c.itemCalls, req)
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

func TestSyncDryRunDetectsChampionButDoesNotApplyLCU(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID:   240,
			Role:         "top",
			QueueID:      420,
			IsAutofilled: false,
		},
	}
	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.DetectedChampionID != 240 {
		t.Fatalf("unexpected detected champion id: %d", got.DetectedChampionID)
	}
	if got.DetectedRole != "top" {
		t.Fatalf("unexpected detected role: %q", got.DetectedRole)
	}
	if got.DetectedQueueID != 420 {
		t.Fatalf("unexpected detected queue id: %d", got.DetectedQueueID)
	}

	if got.ItemSetApplied || got.RunePageApplied || got.SpellsApplied {
		t.Fatalf("expected no applied flags in dry-run, got %#v", got)
	}

	if lcu.detectCalls != 1 {
		t.Fatalf("expected exactly one detect call, got %d", lcu.detectCalls)
	}

	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("expected no LCU apply calls in dry-run, got items=%d runes=%d spells=%d", len(lcu.itemSetCalls), len(lcu.runePageCalls), len(lcu.spellCalls))
	}

	if coachless.getPatchesCalls != 1 {
		t.Fatalf("expected coachless calls after successful detect, got getPatches=%d", coachless.getPatchesCalls)
	}
	if len(coachless.keystoneCalls) != 1 {
		t.Fatalf("expected one keystone call, got %d", len(coachless.keystoneCalls))
	}
	if coachless.keystoneCalls[0].CommonFilters.Role != 0 {
		t.Fatalf("expected detected top role code 0, got %d", coachless.keystoneCalls[0].CommonFilters.Role)
	}
}

func TestSyncUsesDetectedChampionIDInApplyRequests(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID:   777,
			Role:         "support",
			QueueID:      440,
			IsAutofilled: true,
		},
	}
	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.DetectedChampionID != 777 {
		t.Fatalf("expected detected champion 777, got %d", got.DetectedChampionID)
	}
	if got.DetectedRole != "support" {
		t.Fatalf("expected detected role support, got %q", got.DetectedRole)
	}
	if got.DetectedQueueID != 440 {
		t.Fatalf("expected detected queue 440, got %d", got.DetectedQueueID)
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("expected warnings to include patch/autofill messages")
	}

	if len(lcu.itemSetCalls) != 1 || len(lcu.runePageCalls) != 1 || len(lcu.spellCalls) != 1 {
		t.Fatalf("unexpected LCU apply calls: items=%d runes=%d spells=%d", len(lcu.itemSetCalls), len(lcu.runePageCalls), len(lcu.spellCalls))
	}

	if lcu.itemSetCalls[0].ChampionID != 777 || lcu.runePageCalls[0].ChampionID != 777 || lcu.spellCalls[0].ChampionID != 777 {
		t.Fatalf("apply calls must use detected champion id")
	}
	if lcu.itemSetCalls[0].Role != "support" || lcu.runePageCalls[0].Role != "support" || lcu.spellCalls[0].Role != "support" {
		t.Fatalf("apply calls must use detected role")
	}
	if len(coachless.keystoneCalls) != 1 {
		t.Fatalf("expected one keystone call, got %d", len(coachless.keystoneCalls))
	}
	if coachless.keystoneCalls[0].CommonFilters.Role != 4 {
		t.Fatalf("expected support role code 4, got %d", coachless.keystoneCalls[0].CommonFilters.Role)
	}
	if len(coachless.keystoneCalls[0].CommonFilters.ChampionIDs) != 1 || coachless.keystoneCalls[0].CommonFilters.ChampionIDs[0] != 777 {
		t.Fatalf("coachless filters must include detected champion id 777")
	}
}

func TestSyncFailsFastWhenChampionDetectionFails(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{detectErr: errors.New("no session")}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      false,
	})
	if err == nil {
		t.Fatal("expected sync error when detect champion id fails")
	}

	if lcu.detectCalls != 1 {
		t.Fatalf("expected one detect call, got %d", lcu.detectCalls)
	}

	if coachless.getPatchesCalls != 0 {
		t.Fatalf("coachless should not be called when detection fails, got getPatches=%d", coachless.getPatchesCalls)
	}

	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("LCU apply should not run when detection fails")
	}
}
