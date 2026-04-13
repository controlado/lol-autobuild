package lcu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type connectionInfo struct {
	Port     int
	Password string
	Protocol string
}

type champSelectSession struct {
	LocalPlayerCellID int                          `json:"localPlayerCellId"`
	QueueID           int                          `json:"queueId"`
	MyTeam            []champSelectPlayerSelection `json:"myTeam"`
}

type champSelectPlayerSelection struct {
	CellID           int    `json:"cellId"`
	ChampionID       int    `json:"championId"`
	AssignedPosition string `json:"assignedPosition"`
	IsAutofilled     bool   `json:"isAutofilled"`
	Spell1ID         int    `json:"spell1Id"`
	Spell2ID         int    `json:"spell2Id"`
}

type champSelectMySelectionPatch struct {
	Spell1ID int `json:"spell1Id"`
	Spell2ID int `json:"spell2Id"`
}

type Client struct {
	Enabled             bool
	LockfilePath        string
	HTTPClient          *http.Client
	WatchReconnectDelay time.Duration

	discoverProcessConnections func(context.Context) []connectionCandidate
}

func NewClient(enabled bool, lockfilePath string) *Client {
	return &Client{
		Enabled:                    enabled,
		LockfilePath:               strings.TrimSpace(lockfilePath),
		WatchReconnectDelay:        time.Second,
		discoverProcessConnections: discoverProcessConnections,
	}
}

func withLastCandidateError(base error, last error) error {
	if last == nil {
		return base
	}

	return errors.Join(base, fmt.Errorf("last candidate error: %w", last))
}
