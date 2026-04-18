package lcu

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type queueID int

const (
	queueDraftPick       queueID = 400
	queueSoloDuo         queueID = 420
	queueFlex            queueID = 440
	queueCustomDraftPick queueID = 3110
)

func (c *Client) DetectSelection(ctx context.Context) (detectedSelection ports.DetectedSelection, err error) {
	if !c.Enabled {
		return ports.DetectedSelection{}, ErrNotConfigured
	}

	var (
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (shouldTerminate bool) {
			session, err := c.fetchChampSelectSession(ctx, info)
			if err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			detectedSelection, err = selectionFromSession(session)
			if err != nil {
				attempt.observe(candidateLabel, classifyDetectSelectionError(err), err)
				return false
			}

			return true
		}
	)

	if success, err := c.forEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return ports.DetectedSelection{}, err
	} else if success {
		return detectedSelection, nil
	}

	return ports.DetectedSelection{}, attempt.finish(
		ErrChampSelectUnavailable,
		ErrChampionNotSelected,
		ErrRoleNotAssigned,
		ErrRoleUnknown,
		ErrRoleDetectionUnsupportedQueue,
	)
}

func classifyDetectSelectionError(err error) error {
	switch {
	case errors.Is(err, ErrChampionNotSelected):
		return ErrChampionNotSelected
	case errors.Is(err, ErrRoleNotAssigned):
		return ErrRoleNotAssigned
	case errors.Is(err, ErrRoleUnknown):
		return ErrRoleUnknown
	case errors.Is(err, ErrRoleDetectionUnsupportedQueue):
		return ErrRoleDetectionUnsupportedQueue
	default:
		return nil
	}
}

func selectionFromSession(session champSelectSession) (ports.DetectedSelection, error) {
	if !isRoleDetectionQueueSupported(session.QueueID) {
		return ports.DetectedSelection{}, fmt.Errorf("%w: queueId %d", ErrRoleDetectionUnsupportedQueue, session.QueueID)
	}

	member, err := localPlayerFromSession(session)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	if member.ChampionID <= 0 {
		return ports.DetectedSelection{}, ErrChampionNotSelected
	}

	role, err := normalizeAssignedRole(member.AssignedPosition)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	return ports.DetectedSelection{
		ChampionID:   member.ChampionID,
		Role:         role,
		QueueID:      session.QueueID,
		IsAutofilled: member.IsAutofilled,
	}, nil
}

func isRoleDetectionQueueSupported(queueIDValue int) bool {
	switch queueID(queueIDValue) {
	case queueDraftPick, queueSoloDuo, queueFlex, queueCustomDraftPick:
		return true
	default:
		return false
	}
}

func normalizeAssignedRole(assignedPosition string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(assignedPosition)) {
	case "TOP":
		return "top", nil
	case "JUNGLE":
		return "jungle", nil
	case "MIDDLE":
		return "mid", nil
	case "BOTTOM":
		return "adc", nil
	case "UTILITY":
		return "support", nil
	case "", "FILL", "UNSELECTED":
		return "", fmt.Errorf("%w: assignedPosition %q", ErrRoleNotAssigned, assignedPosition)
	default:
		return "", fmt.Errorf("%w: assignedPosition %q", ErrRoleUnknown, assignedPosition)
	}
}
