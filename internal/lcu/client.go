package lcu

import (
	"context"
	"net/http"
	"strings"
	"time"
)

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
