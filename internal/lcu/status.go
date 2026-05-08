package lcu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

type ConnectionState string

const (
	ConnectionStateOff          ConnectionState = "off"
	ConnectionStateNotConnected ConnectionState = "not_connected"
	ConnectionStateConnected    ConnectionState = "connected"
)

type ConnectionStatus struct {
	State  ConnectionState `json:"state"`
	Source string          `json:"source,omitempty"`
	Err    error           `json:"-"`
}

func (c *Client) ConnectionStatus(ctx context.Context) ConnectionStatus {
	if !c.Enabled {
		return ConnectionStatus{State: ConnectionStateOff}
	}

	var (
		status           ConnectionStatus
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (shouldTerminate bool) {
			if err := c.probeConnection(ctx, info); err != nil {
				attempt.observe(candidateLabel, ErrLCUNotReachable, err)
				return false
			}

			status = ConnectionStatus{
				State:  ConnectionStateConnected,
				Source: candidateLabel,
			}
			return true
		}
	)

	if success, err := c.forEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return ConnectionStatus{State: ConnectionStateNotConnected, Err: err}
	} else if success {
		return status
	}

	err := attempt.finish(ErrLockfileNotFound, ErrLCUNotReachable)
	return ConnectionStatus{State: ConnectionStateNotConnected, Err: err}
}

func (c *Client) probeConnection(ctx context.Context, info connectionInfo) error {
	if err := doRequest(ctx, c, info, http.MethodGet, "/riotclient/ux-state", nil); err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return nil
		}
		return fmt.Errorf("probe lcu: %w", err)
	}

	return nil
}
