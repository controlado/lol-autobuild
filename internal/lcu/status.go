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
	State   ConnectionState `json:"state"`
	Message string          `json:"message,omitempty"`
	Source  string          `json:"source,omitempty"`
}

func (c *Client) ConnectionStatus(ctx context.Context) ConnectionStatus {
	if !c.Enabled {
		return ConnectionStatus{
			State:   ConnectionStateOff,
			Message: "LCU is off",
		}
	}

	var (
		status           ConnectionStatus
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (shouldTerminate bool) {
			if err := c.probeConnection(ctx, info); err != nil {
				attempt.observe(candidateLabel, nil, err)
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
		return ConnectionStatus{
			State:   ConnectionStateNotConnected,
			Message: err.Error(),
		}
	} else if success {
		return status
	}

	return ConnectionStatus{
		State:   ConnectionStateNotConnected,
		Message: attempt.finish(ErrLockfileNotFound).Error(),
	}
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
