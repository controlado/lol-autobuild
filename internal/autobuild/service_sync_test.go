package autobuild

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
	"github.com/controlado/lol-autobuild/internal/autobuild/recommend"
)

func TestSyncDryRunDetectsChampionButDoesNotApplyLCU(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   240,
			ChampionName: "Kled",
			Position:     domain.Top,
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
	if got.DetectedChampionName != "Kled" {
		t.Fatalf("unexpected detected champion name: %q", got.DetectedChampionName)
	}
	if got.DetectedPosition != domain.Top.String() {
		t.Fatalf("unexpected detected position: %q", got.DetectedPosition)
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
	if len(coachless.treeCalls)+len(coachless.runeCalls)+len(coachless.shardCalls) != 0 {
		t.Fatalf("dry-run should not load full rune page data, got trees=%d runes=%d shards=%d", len(coachless.treeCalls), len(coachless.runeCalls), len(coachless.shardCalls))
	}
	if coachless.keystoneCalls[0].CommonFilters.Role != 0 {
		t.Fatalf("expected detected top role code 0, got %d", coachless.keystoneCalls[0].CommonFilters.Role)
	}
}

func TestSyncUsesDetectedChampionIDInApplyRequests(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   777,
			ChampionName: "Yone",
			Position:     domain.Support,
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

	wantPosition := domain.Support
	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		KeepFlash:   true,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.DetectedChampionID != 777 {
		t.Fatalf("expected detected champion 777, got %d", got.DetectedChampionID)
	}
	if got.DetectedChampionName != "Yone" {
		t.Fatalf("expected detected champion Yone, got %q", got.DetectedChampionName)
	}
	if got.DetectedPosition != wantPosition.String() {
		t.Fatalf("expected detected position support, got %q", got.DetectedPosition)
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
	if lcu.itemSetCalls[0].ChampionName != "Yone" || lcu.runePageCalls[0].ChampionName != "Yone" {
		t.Fatalf("item and rune apply calls must use detected champion name")
	}
	if lcu.itemSetCalls[0].Position != wantPosition || lcu.runePageCalls[0].Position != wantPosition || lcu.spellCalls[0].Position != wantPosition {
		t.Fatalf("apply calls must use detected role")
	}
	if !lcu.spellCalls[0].KeepFlash {
		t.Fatalf("summoner spell apply should keep flash")
	}
	wantPage := domain.RunePage{
		PrimaryStyleID:  domain.RuneStyleResolve,
		SubStyleID:      domain.RuneStyleSorcery,
		SelectedPerkIDs: []int{8437, 8446, 8444, 8451, 8233, 8224, 5008, 5008, 5002},
	}
	if !reflect.DeepEqual(lcu.runePageCalls[0].Page, wantPage) {
		t.Fatalf("unexpected rune page: %#v", lcu.runePageCalls[0].Page)
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
	if len(coachless.treeCalls) != 1 || len(coachless.runeCalls) != 2 || len(coachless.shardCalls) != 1 {
		t.Fatalf("expected complete rune page lookups, got trees=%d runes=%d shards=%d", len(coachless.treeCalls), len(coachless.runeCalls), len(coachless.shardCalls))
	}
}

func TestSyncTranslatesRunePageLimitWarning(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Top,
			QueueID:    420,
		},
		runePageErr: fmt.Errorf(`apply rune page failed: {"errorCode":"RPC_ERROR","message":"Max pages reached"}: %w`, domain.ErrRunePageLimitReached),
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
		ApplyRunes: true,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.RunePageApplied {
		t.Fatalf("expected rune page apply to fail, got %#v", got)
	}
	if len(lcu.runePageCalls) != 1 {
		t.Fatalf("expected one rune page apply call, got %d", len(lcu.runePageCalls))
	}

	foundLimitWarning := false
	for _, warning := range got.Warnings {
		if warning == RunePageLimitReachedWarning {
			foundLimitWarning = true
		}
		if strings.Contains(warning, "RPC_ERROR") || strings.Contains(warning, "Max pages reached") {
			t.Fatalf("expected translated warning without raw LCU body, got %#v", got.Warnings)
		}
	}
	if !foundLimitWarning {
		t.Fatalf("expected rune page limit warning, got %#v", got.Warnings)
	}
}

func TestSyncDoesNotApplyIncompleteRunePage(t *testing.T) {
	t.Parallel()

	emptyPrimaryRunes := domain.RuneStatsByRow{
		RowTwos:   []domain.RuneStat{{Rune: 8444, WPAOverall: 0.8, Occurrence: 1000}},
		RowThrees: []domain.RuneStat{{Rune: 8451, WPAOverall: 0.7, Occurrence: 1000}},
	}
	coachless := &coachlessStub{primaryRunes: &emptyPrimaryRunes}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Top,
			QueueID:    420,
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
		ApplyRunes: true,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.RunePageApplied {
		t.Fatalf("expected incomplete rune page to skip apply, got %#v", got)
	}
	if len(lcu.runePageCalls) != 0 {
		t.Fatalf("expected no rune page apply call, got %d", len(lcu.runePageCalls))
	}
	if len(coachless.treeCalls) != 1 || len(coachless.runeCalls) != 2 || len(coachless.shardCalls) != 1 {
		t.Fatalf("expected complete rune page lookup before validation, got trees=%d runes=%d shards=%d", len(coachless.treeCalls), len(coachless.runeCalls), len(coachless.shardCalls))
	}

	foundWarning := false
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "no primary row 1 rune recommendation") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected missing rune row warning, got %#v", got.Warnings)
	}
}

func TestSyncUsesAdvancedCoachlessFilters(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{
		patches: []domain.PatchInfo{
			{Label: "16.6", Major: 16, Patch: 6},
			{Label: "16.7", Major: 16, Patch: 7},
			{Label: "16.8", Major: 16, Patch: 8},
		},
	}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Top,
			QueueID:    420,
		},
	}
	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t", claims: domain.TokenClaims{Subscribed: true}},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.Sync(context.Background(), SyncRequest{
		PatchAdditionsMode: PatchAdditionsModeManual,
		PatchAdditions:     PatchAdditionsDefault,
		LeagueTierPreset:   LeagueTierPresetDiamondPlus,
		ApplyItems:         true,
		ApplyRunes:         true,
		ApplySpells:        true,
		DryRun:             true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(coachless.keystoneCalls) != 1 {
		t.Fatalf("expected one keystone call, got %d", len(coachless.keystoneCalls))
	}

	gotFilters := coachless.keystoneCalls[0].CommonFilters
	if gotFilters.Patch.Major != 16 || gotFilters.Patch.Patch != 8 || gotFilters.Patch.PatchAdditions != PatchAdditionsDefault {
		t.Fatalf("unexpected patch filter: %#v", gotFilters.Patch)
	}
	if !slices.Equal(gotFilters.LeagueTiers, []int{6, 7}) {
		t.Fatalf("league tiers = %#v, want %#v", gotFilters.LeagueTiers, []int{6, 7})
	}
}

func TestSyncSendsSelectedMatchupsInLCUEnemyOrder(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Top,
			QueueID:    420,
			EnemyChampions: []domain.ChampionRef{
				{ID: 10, Name: "Kayle"},
				{ID: 20, Name: "Nunu & Willump"},
				{ID: 30, Name: "Ashe"},
				{ID: 40, Name: "Jhin"},
				{ID: 50, Name: "Karma"},
				{ID: 60, Name: "Elise"},
			},
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

	_, err = svc.Sync(context.Background(), SyncRequest{
		MatchupChampionIDs: []int{60, 20, 0, 20, 10, 30, 40, 50},
		ApplyItems:         true,
		ApplyRunes:         true,
		ApplySpells:        true,
		DryRun:             false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	want := []int{10, 20, 30, 40, 60}
	assertMatchups(t, coachless.keystoneCalls[0].CommonFilters, want)
	assertMatchups(t, coachless.spellCalls[0].CommonFilters, want)
	for idx, call := range coachless.itemCalls {
		assertMatchupsFor(t, call.CommonFilters, want, fmt.Sprintf("item call %d", idx))
	}
	for idx, call := range coachless.treeCalls {
		assertMatchupsFor(t, call.CommonFilters, want, fmt.Sprintf("tree call %d", idx))
	}
	for idx, call := range coachless.runeCalls {
		assertMatchupsFor(t, call.CommonFilters, want, fmt.Sprintf("rune call %d", idx))
	}
	for idx, call := range coachless.shardCalls {
		assertMatchupsFor(t, call.CommonFilters, want, fmt.Sprintf("shard call %d", idx))
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

func TestSyncReturnsErrorWhenRecommendationQueryFails(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("spell endpoint failed")
	coachless := &coachlessStub{spellErr: queryErr}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   240,
			Position:     domain.Top,
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

	_, err = svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      false,
	})
	if err == nil {
		t.Fatal("expected sync error when one recommendation query fails")
	}
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected wrapped query error, got %v", err)
	}

	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("LCU apply should not run when recommendation query fails")
	}
}

func TestSyncBuildsCoachlessStyleItemBlocks(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   777,
			Position:     domain.Support,
			QueueID:      440,
			IsAutofilled: false,
		},
	}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 1, TopSpells: 2},
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
	if !got.ItemSetApplied {
		t.Fatalf("expected item set to be applied, got %#v", got)
	}
	if len(lcu.itemSetCalls) != 1 {
		t.Fatalf("expected one item set apply call, got %d", len(lcu.itemSetCalls))
	}

	blocks := lcu.itemSetCalls[0].Blocks
	wantBlockTypes := []string{"Starter", "1st Item", "2nd Item", "Boots", "3rd Item", "4th+ Item"}
	if len(blocks) != len(wantBlockTypes) {
		t.Fatalf("expected %d blocks, got %d (%#v)", len(wantBlockTypes), len(blocks), blocks)
	}
	for idx, block := range blocks {
		if block.Type != wantBlockTypes[idx] {
			t.Fatalf("unexpected block order/type at index %d: got %q want %q", idx, block.Type, wantBlockTypes[idx])
		}
		if len(block.ItemIDs) != 1 || block.ItemIDs[0] != 1055 {
			t.Fatalf("unexpected block items at index %d: %#v", idx, block.ItemIDs)
		}
	}

	if len(coachless.itemCalls) != 6 {
		t.Fatalf("expected 6 staged item calls, got %d", len(coachless.itemCalls))
	}
	if !hasItemCall(coachless.itemCalls, 6, nil, false) {
		t.Fatalf("missing starter item query in %+v", coachless.itemCalls)
	}
	if !hasItemCall(coachless.itemCalls, 3, []int{1}, true) {
		t.Fatalf("missing support-first-item query in %+v", coachless.itemCalls)
	}
	if !hasItemCall(coachless.itemCalls, 1, []int{2}, false) {
		t.Fatalf("missing 2nd-item query in %+v", coachless.itemCalls)
	}
	if !hasItemCall(coachless.itemCalls, 2, nil, false) {
		t.Fatalf("missing boots query in %+v", coachless.itemCalls)
	}
	if !hasItemCall(coachless.itemCalls, 1, []int{3}, false) {
		t.Fatalf("missing 3rd-item query in %+v", coachless.itemCalls)
	}
	if !hasItemCall(coachless.itemCalls, 1, []int{4, 5, 6}, false) {
		t.Fatalf("missing 4th+-item query in %+v", coachless.itemCalls)
	}
}

func TestSyncAllowsUnlimitedItemsPerBlock(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{
		itemStats: []domain.ItemStat{
			{ItemID: 1055, WPAOverall: 0.9, Occurrence: 2000},
			{ItemID: 3006, WPAOverall: 0.8, Occurrence: 2000},
			{ItemID: 3031, WPAOverall: 0.7, Occurrence: 2000},
			{ItemID: 2021, WPAOverall: 0.3, Occurrence: 2000},
			{ItemID: 1051, WPAOverall: 0.5, Occurrence: 2000},
			{ItemID: 4032, WPAOverall: 0.4, Occurrence: 2000},
			{ItemID: 5004, WPAOverall: 0.6, Occurrence: 2000},
		},
	}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   235,
			Position:     domain.ADC,
			QueueID:      420,
			IsAutofilled: false,
		},
	}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 1000, TopItems: 0, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems: true,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !got.ItemSetApplied {
		t.Fatalf("expected item set to be applied, got %#v", got)
	}
	if len(lcu.itemSetCalls) != 1 {
		t.Fatalf("expected one item set apply call, got %d", len(lcu.itemSetCalls))
	}

	for idx, block := range lcu.itemSetCalls[0].Blocks {
		if len(block.ItemIDs) != 7 {
			t.Fatalf("expected unlimited block items at index %d, got %#v", idx, block.ItemIDs)
		}
	}
}

func TestSyncFallsBackToRawItemsWhenOccurrenceFilterWouldEmptyBlock(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{
		itemStats: []domain.ItemStat{
			{ItemID: 3142, WPAOverall: 1.2, Occurrence: 100},
			{ItemID: 3087, WPAOverall: 0.9, Occurrence: 200},
		},
	}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   235,
			Position:     domain.ADC,
			QueueID:      420,
			IsAutofilled: false,
		},
	}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 1000, TopItems: 0, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems: true,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !got.ItemSetApplied {
		t.Fatalf("expected item set to be applied, got %#v", got)
	}

	for idx, block := range lcu.itemSetCalls[0].Blocks {
		if len(block.ItemIDs) != 2 || block.ItemIDs[0] != 3142 || block.ItemIDs[1] != 3087 {
			t.Fatalf("expected raw fallback items at index %d, got %#v", idx, block.ItemIDs)
		}
	}
}

func TestResolvePatch(t *testing.T) {
	t.Parallel()

	defaultPatches := []domain.PatchInfo{
		{Label: "16.4", Major: 16, Patch: 4},
		{Label: "16.5", Major: 16, Patch: 5},
		{Label: "16.6", Major: 16, Patch: 6},
		{Label: "16.7", Major: 16, Patch: 7},
		{Label: "16.8", Major: 16, Patch: 8},
	}

	tests := []struct {
		name             string
		rawPatch         string
		mode             string
		additions        int
		patches          []domain.PatchInfo
		subscribed       bool
		wantLabel        string
		wantPatch        int
		wantAdditions    int
		wantWarningCount int
		wantErr          bool
	}{
		{name: "free auto selects latest non-premium patch", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantLabel: "16.7", wantPatch: 7},
		{name: "premium auto selects latest patch with two additions", mode: PatchAdditionsModeAuto, patches: defaultPatches, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsDefault},
		{name: "premium manual current patch", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 0, subscribed: true, wantLabel: "16.8", wantPatch: 8},
		{name: "premium manual one addition", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 1, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: 1},
		{name: "premium manual two additions", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: PatchAdditionsDefault, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsDefault},
		{name: "premium manual four additions", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: PatchAdditionsMax, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsMax},
		{name: "free manual additions require premium", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 1, wantErr: true},
		{name: "explicit older patch is allowed for free", rawPatch: "16.6", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantLabel: "16.6", wantPatch: 6},
		{name: "explicit latest patch requires premium for free", rawPatch: "16.8", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantErr: true},
		{name: "manual additions clamp to available history", mode: PatchAdditionsModeManual, patches: defaultPatches[:2], additions: PatchAdditionsMax, subscribed: true, wantLabel: "16.5", wantPatch: 5, wantAdditions: 1, wantWarningCount: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, label, warnings, err := resolvePatch(tt.rawPatch, tt.mode, tt.additions, tt.patches, tt.subscribed)
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolvePatch() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePatch() error = %v", err)
			}
			if label != tt.wantLabel {
				t.Fatalf("label = %q, want %q", label, tt.wantLabel)
			}
			if got.Patch != tt.wantPatch || got.PatchAdditions != tt.wantAdditions {
				t.Fatalf("filter = %#v, want patch=%d additions=%d", got, tt.wantPatch, tt.wantAdditions)
			}
			if len(warnings) != tt.wantWarningCount {
				t.Fatalf("warnings = %#v, want count %d", warnings, tt.wantWarningCount)
			}
		})
	}
}

func TestResolveLeagueTierPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		preset  string
		want    []int
		wantErr bool
	}{
		{name: "empty defaults to emerald plus", want: []int{5, 6, 7}},
		{name: "gold plus", preset: LeagueTierPresetGoldPlus, want: []int{3, 4, 5, 6, 7}},
		{name: "platinum plus", preset: LeagueTierPresetPlatinumPlus, want: []int{4, 5, 6, 7}},
		{name: "emerald plus", preset: LeagueTierPresetEmeraldPlus, want: []int{5, 6, 7}},
		{name: "diamond plus", preset: LeagueTierPresetDiamondPlus, want: []int{6, 7}},
		{name: "master plus", preset: LeagueTierPresetMasterPlus, want: []int{7}},
		{name: "invalid", preset: "silver_plus", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveLeagueTierPreset(tt.preset)
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolveLeagueTierPreset() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveLeagueTierPreset() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("tiers = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func hasItemCall(calls []domain.ItemStatsRequest, itemType int, itemSlots []int, includeSupportItems bool) bool {
	return slices.ContainsFunc(calls, func(call domain.ItemStatsRequest) bool {
		return call.ItemType == itemType &&
			slices.Equal(call.ItemSlots, itemSlots) &&
			call.IncludeSupportItems == includeSupportItems
	})
}

func assertMatchups(t *testing.T, filters domain.CommonFilters, want []int) {
	t.Helper()

	if slices.Equal(filters.MatchupChampionIDs, want) {
		return
	}

	t.Fatalf("MatchupChampionIDs = %+v, want %+v", filters.MatchupChampionIDs, want)
}

func assertMatchupsFor(t *testing.T, filters domain.CommonFilters, want []int, label string) {
	t.Helper()

	if slices.Equal(filters.MatchupChampionIDs, want) {
		return
	}

	t.Fatalf("%s MatchupChampionIDs = %+v, want %+v", label, filters.MatchupChampionIDs, want)
}
