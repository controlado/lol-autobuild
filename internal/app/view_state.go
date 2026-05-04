package app

import (
	"time"
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

type LCUConnectionState string

const (
	LCUConnectionStateOff          LCUConnectionState = "off"
	LCUConnectionStateNotConnected LCUConnectionState = "not_connected"
	LCUConnectionStateConnected    LCUConnectionState = "connected"
)

type (
	LCUStatus struct {
		State   LCUConnectionState `json:"state"`
		Message string             `json:"message,omitempty"`
		Source  string             `json:"source,omitempty"`
	}
	SyncSummary struct {
		DetectedChampionID   int      `json:"DetectedChampionID"`
		DetectedChampionName string   `json:"DetectedChampionName"`
		DetectedPosition     string   `json:"DetectedPosition"`
		DetectedQueueID      int      `json:"DetectedQueueID"`
		ItemSetApplied       bool     `json:"ItemSetApplied"`
		RunePageApplied      bool     `json:"RunePageApplied"`
		SpellsApplied        bool     `json:"SpellsApplied"`
		Warnings             []string `json:"Warnings"`
	}
	WatcherNoticeState struct {
		Kind         string    `json:"kind"`
		Message      string    `json:"message,omitempty"`
		Error        string    `json:"error,omitempty"`
		Source       string    `json:"source,omitempty"`
		URI          string    `json:"uri,omitempty"`
		Phase        string    `json:"phase,omitempty"`
		ConnectionID int       `json:"connection_id,omitempty"`
		At           time.Time `json:"at"`
	}
	WatcherState struct {
		Running     bool                `json:"running"`
		ConfigStale bool                `json:"config_stale"`
		LastNotice  *WatcherNoticeState `json:"last_notice,omitempty"`
	}
	ViewState struct {
		Settings      Settings     `json:"settings"`
		LCU           LCUStatus    `json:"lcu"`
		Watcher       WatcherState `json:"watcher"`
		Update        UpdateState  `json:"update"`
		SyncRunning   bool         `json:"sync_running"`
		LastSync      *SyncSummary `json:"last_sync,omitempty"`
		LastSyncAt    *time.Time   `json:"last_sync_at,omitempty"`
		LastError     string       `json:"last_error,omitempty"`
		LastErrorCode string       `json:"last_error_code,omitempty"`
	}
)
