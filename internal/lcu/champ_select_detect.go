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

func (c *Client) DetectSelection(ctx context.Context) (ports.DetectedSelection, error) {
	if !c.Enabled {
		return ports.DetectedSelection{}, ErrNotConfigured
	}

	var attempt = newConnectionAttempt()
	for _, candidate := range c.candidates(ctx) {
		if err := ctx.Err(); err != nil {
			return ports.DetectedSelection{}, err
		}

		info, err := candidate.resolve()
		if err != nil {
			if !errors.Is(err, ErrLockfileNotFound) {
				attempt.markResolvableCandidate()
			}
			attempt.observe(candidate.label(), ErrLockfileNotFound, err)
			continue
		}
		attempt.markResolvableCandidate()

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			attempt.observe(candidate.label(), nil, err)
			continue
		}

		selection, err := selectionFromSession(session)
		if err != nil {
			attempt.observe(candidate.label(), classifyDetectSelectionError(err), err)
			continue
		}

		return selection, nil
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
