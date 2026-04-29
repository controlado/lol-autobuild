package app

import (
	"time"

	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/pkg/lolautobuild"
)

type UpdateStatus string

const (
	UpdateStatusIdle        UpdateStatus = "idle"
	UpdateStatusChecking    UpdateStatus = "checking"
	UpdateStatusCurrent     UpdateStatus = "current"
	UpdateStatusAvailable   UpdateStatus = "available"
	UpdateStatusError       UpdateStatus = "error"
	UpdateStatusUnavailable UpdateStatus = "unavailable"
)

type UpdateState struct {
	Status         UpdateStatus `json:"status"`
	CurrentVersion string       `json:"current_version,omitempty"`
	LatestVersion  string       `json:"latest_version,omitempty"`
	DownloadURL    string       `json:"download_url,omitempty"`
	CheckedAt      *time.Time   `json:"checked_at,omitempty"`
	Message        string       `json:"message,omitempty"`
}

type (
	WatcherState struct {
		Running bool `json:"running"`
	}
	State struct {
		Settings      Settings                 `json:"settings"`
		LCU           lcu.ConnectionStatus     `json:"lcu"`
		Watcher       WatcherState             `json:"watcher"`
		Update        UpdateState              `json:"update"`
		SyncRunning   bool                     `json:"sync_running"`
		LastSync      *lolautobuild.SyncResult `json:"last_sync,omitempty"`
		LastSyncAt    *time.Time               `json:"last_sync_at,omitempty"`
		LastError     string                   `json:"last_error,omitempty"`
		LastErrorCode string                   `json:"last_error_code,omitempty"`
	}
)
