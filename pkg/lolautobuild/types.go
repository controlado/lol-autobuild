package lolautobuild

import (
	"context"
	"time"
)

type Service interface {
	EnsureCoachlessAuth(ctx context.Context) error
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
	Watch(ctx context.Context, req WatchRequest) error
}

const (
	PatchAdditionsModeAuto   = "auto"
	PatchAdditionsModeManual = "manual"
	PatchAdditionsDefault    = 2
	PatchAdditionsMax        = 4
)

const (
	LeagueTierPresetGoldPlus     = "gold_plus"
	LeagueTierPresetPlatinumPlus = "platinum_plus"
	LeagueTierPresetEmeraldPlus  = "emerald_plus"
	LeagueTierPresetDiamondPlus  = "diamond_plus"
	LeagueTierPresetMasterPlus   = "master_plus"
	LeagueTierPresetDefault      = LeagueTierPresetEmeraldPlus
)

type SyncRequest struct {
	Patch              string
	PatchAdditionsMode string
	PatchAdditions     int
	LeagueTierPreset   string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	KeepFlash   bool

	DryRun bool
}

type SyncResult struct {
	DetectedChampionID int
	DetectedPosition   string
	DetectedQueueID    int
	ItemSetApplied     bool
	RunePageApplied    bool
	SpellsApplied      bool
	Warnings           []string
}

type WatchRequest struct {
	Patch              string
	PatchAdditionsMode string
	PatchAdditions     int
	LeagueTierPreset   string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	KeepFlash   bool

	DryRun bool

	Debounce time.Duration
	OnCycle  func(WatchCycle)
}

type WatchTrigger string

const (
	WatchTriggerStartup WatchTrigger = "startup"
	WatchTriggerEvent   WatchTrigger = "event"
)

type WatchCycle struct {
	Trigger   WatchTrigger
	EventType string
	EventURI  string
	Result    *SyncResult
	Err       error
}
