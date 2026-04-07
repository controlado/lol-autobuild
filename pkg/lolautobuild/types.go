package lolautobuild

import "context"

type Service interface {
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
}

type SyncRequest struct {
	ChampionID int
	Role       string
	Patch      string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool

	DryRun bool
}

type SyncResult struct {
	ItemSetApplied  bool
	RunePageApplied bool
	SpellsApplied   bool
	Warnings        []string
}
