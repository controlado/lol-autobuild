package lolautobuild

import (
	"context"
	"errors"
	"testing"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/recommend"
)

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
