package lcu

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Enabled             bool
	LockfilePath        string
	HTTPClient          *http.Client
	WatchReconnectDelay time.Duration

	discoverProcessConnections func(context.Context) []connectionCandidate

	championNameMu        sync.Mutex
	championNameCache     map[int]string
	championSummaryLoaded bool
}

func NewClient(enabled bool, lockfilePath string) *Client {
	return &Client{
		Enabled:                    enabled,
		LockfilePath:               strings.TrimSpace(lockfilePath),
		WatchReconnectDelay:        time.Second,
		discoverProcessConnections: discoverProcessConnections,
	}
}
