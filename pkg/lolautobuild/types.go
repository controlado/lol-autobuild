package lolautobuild

import (
	"context"
	"time"
)

type Service interface {
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
	Watch(ctx context.Context, req WatchRequest) error
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

type WatchRequest struct {
	Patch string

	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool

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
