package autobuild

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
	Regions            []int
	MatchupChampionIDs []int

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	KeepFlash   bool

	DryRun bool
}

type SyncResult struct {
	DetectedChampionID   int
	DetectedChampionName string
	DetectedPosition     string
	DetectedQueueID      int
	ItemSetApplied       bool
	RunePageApplied      bool
	SpellsApplied        bool
	Warnings             []string
}

type WatchRequest struct {
	Patch                      string
	PatchAdditionsMode         string
	PatchAdditions             int
	LeagueTierPreset           string
	Regions                    []int
	SelectedMatchupChampionIDs func() []int

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	KeepFlash   bool

	DryRun bool

	Debounce time.Duration
	OnCycle  func(WatchCycle)
	OnNotice func(WatchNotice)
}

type WatchTrigger string

const (
	WatchTriggerEvent    WatchTrigger = "event"
	WatchTriggerSnapshot WatchTrigger = "snapshot"
)

type WatchCycle struct {
	Trigger   WatchTrigger
	EventType string
	EventURI  string
	Result    *SyncResult
	Err       error
}

type WatchNoticeKind string

const (
	WatchNoticeConnected            WatchNoticeKind = "connected"
	WatchNoticeReconnecting         WatchNoticeKind = "reconnecting"
	WatchNoticeSnapshotFinalization WatchNoticeKind = "snapshot_finalization"
	WatchNoticeSnapshotWaiting      WatchNoticeKind = "snapshot_waiting"
)

type WatchNotice struct {
	Kind         WatchNoticeKind
	Message      string
	Err          error
	Source       string
	URI          string
	Phase        string
	ConnectionID int
}
