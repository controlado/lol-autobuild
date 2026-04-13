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

	var lastErr error
	seenChampionNotSelected := false
	seenRoleNotAssigned := false
	seenRoleUnknown := false
	seenUnsupportedQueue := false
	seenSessionUnavailable := false
	seenConnection := false

	for _, candidate := range c.candidates(ctx) {
		info, err := candidate.resolve()
		if err != nil {
			if !errors.Is(err, ErrLockfileNotFound) {
				seenConnection = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}
		seenConnection = true

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		selection, err := selectionFromSession(session)
		if err != nil {
			if errors.Is(err, ErrChampionNotSelected) {
				seenChampionNotSelected = true
			}
			if errors.Is(err, ErrRoleNotAssigned) {
				seenRoleNotAssigned = true
			}
			if errors.Is(err, ErrRoleUnknown) {
				seenRoleUnknown = true
			}
			if errors.Is(err, ErrRoleDetectionUnsupportedQueue) {
				seenUnsupportedQueue = true
			}
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		return selection, nil
	}

	if seenChampionNotSelected {
		return ports.DetectedSelection{}, withLastCandidateError(ErrChampionNotSelected, lastErr)
	}

	if seenRoleNotAssigned {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleNotAssigned, lastErr)
	}

	if seenRoleUnknown {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleUnknown, lastErr)
	}

	if seenUnsupportedQueue {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleDetectionUnsupportedQueue, lastErr)
	}

	if seenSessionUnavailable {
		return ports.DetectedSelection{}, withLastCandidateError(ErrChampSelectUnavailable, lastErr)
	}

	if !seenConnection {
		return ports.DetectedSelection{}, ErrLockfileNotFound
	}

	return ports.DetectedSelection{}, withLastCandidateError(ErrLockfileNotFound, lastErr)
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

func localPlayerFromSession(session champSelectSession) (champSelectPlayerSelection, error) {
	for _, member := range session.MyTeam {
		if member.CellID == session.LocalPlayerCellID {
			return member, nil
		}
	}

	return champSelectPlayerSelection{}, fmt.Errorf("%w: local player cell %d not found in myTeam", ErrChampSelectUnavailable, session.LocalPlayerCellID)
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
