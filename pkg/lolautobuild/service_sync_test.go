package lolautobuild

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/position"
	"github.com/controlado/lol-autobuild/internal/recommend"
)

func TestSyncDryRunDetectsChampionButDoesNotApplyLCU(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID:   240,
			Position:     position.Top,
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
	if got.DetectedPosition != position.Top.String() {
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
			Position:     position.Support,
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

	wantPosition := position.Support
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
	if lcu.itemSetCalls[0].Position != wantPosition || lcu.runePageCalls[0].Position != wantPosition || lcu.spellCalls[0].Position != wantPosition {
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

func TestSyncReturnsErrorWhenRecommendationQueryFails(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("spell endpoint failed")
	coachless := &coachlessStub{spellErr: queryErr}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID:   240,
			Position:     position.Top,
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
		detectedSelection: ports.DetectedSelection{
			ChampionID:   777,
			Position:     position.Support,
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
		itemStats: []ports.ItemStat{
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
		detectedSelection: ports.DetectedSelection{
			ChampionID:   235,
			Position:     position.ADC,
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
		itemStats: []ports.ItemStat{
			{ItemID: 3142, WPAOverall: 1.2, Occurrence: 100},
			{ItemID: 3087, WPAOverall: 0.9, Occurrence: 200},
		},
	}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID:   235,
			Position:     position.ADC,
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

func TestResolvePatchHonorsFreePatchLimits(t *testing.T) {
	t.Parallel()

	patches := []ports.PatchInfo{
		{Label: "16.6", Major: 16, Patch: 6},
		{Label: "16.7", Major: 16, Patch: 7},
		{Label: "16.8", Major: 16, Patch: 8},
	}

	got, label, err := resolvePatch("", patches, false)
	if err != nil {
		t.Fatalf("resolvePatch() error = %v", err)
	}
	if label != "16.7" {
		t.Fatalf("expected latest free patch 16.7, got %q", label)
	}
	if got.PatchAdditions != 0 {
		t.Fatalf("free patch additions = %d, want 0", got.PatchAdditions)
	}

	got, label, err = resolvePatch("16.6", patches, false)
	if err != nil {
		t.Fatalf("resolvePatch(explicit free patch) error = %v", err)
	}
	if label != "16.6" || got.Patch != 6 || got.PatchAdditions != 0 {
		t.Fatalf("unexpected explicit free patch: filter=%#v label=%q", got, label)
	}

	_, _, err = resolvePatch("16.8", patches, false)
	if err == nil {
		t.Fatal("expected premium error for newest free patch")
	}
}

func TestResolvePatchAllowsPremiumPatchAdditions(t *testing.T) {
	t.Parallel()

	patches := []ports.PatchInfo{
		{Label: "16.6", Major: 16, Patch: 6},
		{Label: "16.7", Major: 16, Patch: 7},
		{Label: "16.8", Major: 16, Patch: 8},
	}

	got, label, err := resolvePatch("", patches, true)
	if err != nil {
		t.Fatalf("resolvePatch() error = %v", err)
	}
	if label != "16.8" {
		t.Fatalf("expected premium latest patch 16.8, got %q", label)
	}
	if got.PatchAdditions != 2 {
		t.Fatalf("premium patch additions = %d, want 2", got.PatchAdditions)
	}
}

func hasItemCall(calls []ports.ItemStatsRequest, itemType int, itemSlots []int, includeSupportItems bool) bool {
	for _, call := range calls {
		if call.ItemType == itemType &&
			reflect.DeepEqual(call.ItemSlots, itemSlots) &&
			call.IncludeSupportItems == includeSupportItems {
			return true
		}
	}
	return false
}
