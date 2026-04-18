package lcu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
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

func (c *Client) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx

	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply rune page: %w", ErrNotConfigured)
}
