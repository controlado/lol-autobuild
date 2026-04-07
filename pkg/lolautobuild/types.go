package lolautobuild

import "context"

type Service interface {
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
}

type SyncRequest struct {
	Patch string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool

	DryRun bool
}

type SyncResult struct {
	DetectedChampionID int
	DetectedRole       string
	DetectedQueueID    int
	ItemSetApplied     bool
	RunePageApplied    bool
	SpellsApplied      bool
	Warnings           []string
}
