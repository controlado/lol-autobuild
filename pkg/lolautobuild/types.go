package lolautobuild

import "context"

type Service interface {
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
}

type SyncRequest struct {
	Role  string
	Patch string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool

	DryRun bool
}

type SyncResult struct {
	DetectedChampionID int
	ItemSetApplied     bool
	RunePageApplied    bool
	SpellsApplied      bool
	Warnings           []string
}
