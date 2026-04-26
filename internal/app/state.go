package app

import (
	"time"

	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/pkg/lolautobuild"
)

type (
	WatcherState struct {
		Running bool `json:"running"`
	}
	State struct {
		Settings    Settings                 `json:"settings"`
		LCU         lcu.ConnectionStatus     `json:"lcu"`
		Watcher     WatcherState             `json:"watcher"`
		SyncRunning bool                     `json:"sync_running"`
		LastSync    *lolautobuild.SyncResult `json:"last_sync,omitempty"`
		LastSyncAt  *time.Time               `json:"last_sync_at,omitempty"`
		LastError   string                   `json:"last_error,omitempty"`
	}
)
