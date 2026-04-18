package lcu

import (
	"context"
	"errors"
	"fmt"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/position"
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
		position.ErrNotAssigned,
		position.ErrUnknown,
		ErrPositionDetectionUnsupportedQueue,
	)
}

func classifyDetectSelectionError(err error) error {
	switch {
	case errors.Is(err, ErrChampionNotSelected):
		return ErrChampionNotSelected
	case errors.Is(err, position.ErrNotAssigned):
		return position.ErrNotAssigned
	case errors.Is(err, position.ErrUnknown):
		return position.ErrUnknown
	case errors.Is(err, ErrPositionDetectionUnsupportedQueue):
		return ErrPositionDetectionUnsupportedQueue
	default:
		return nil
	}
}

func selectionFromSession(session champSelectSession) (ports.DetectedSelection, error) {
	if !isPositionDetectionQueueSupported(session.QueueID) {
		return ports.DetectedSelection{}, fmt.Errorf("%w: queueId %d", ErrPositionDetectionUnsupportedQueue, session.QueueID)
	}

	member, err := localPlayerFromSession(session)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	if member.ChampionID <= 0 {
		return ports.DetectedSelection{}, ErrChampionNotSelected
	}

	position, err := position.FromRaw(member.AssignedPosition)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	return ports.DetectedSelection{
		ChampionID:   member.ChampionID,
		Position:     position,
		QueueID:      session.QueueID,
		IsAutofilled: member.IsAutofilled,
	}, nil
}

func isPositionDetectionQueueSupported(queueIDValue int) bool {
	switch queueID(queueIDValue) {
	case queueDraftPick, queueSoloDuo, queueFlex, queueCustomDraftPick:
		return true
	default:
		return false
	}
}
