package lcu

import (
	"context"
	"errors"
	"fmt"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type queueID int

const (
	queueDraftPick       queueID = 400
	queueSoloDuo         queueID = 420
	queueFlex            queueID = 440
	queueCustomDraftPick queueID = 3110
)

func (c *Client) DetectSelection(ctx context.Context) (detectedSelection domain.DetectedSelection, err error) {
	if !c.Enabled {
		return domain.DetectedSelection{}, ErrNotConfigured
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
		return domain.DetectedSelection{}, err
	} else if success {
		return detectedSelection, nil
	}

	return domain.DetectedSelection{}, attempt.finish(
		ErrChampSelectUnavailable,
		ErrChampionNotSelected,
		domain.ErrPositionNotAssigned,
		domain.ErrPositionUnknown,
		ErrPositionDetectionUnsupportedQueue,
	)
}

func classifyDetectSelectionError(err error) error {
	switch {
	case errors.Is(err, ErrChampionNotSelected):
		return ErrChampionNotSelected
	case errors.Is(err, domain.ErrPositionNotAssigned):
		return domain.ErrPositionNotAssigned
	case errors.Is(err, domain.ErrPositionUnknown):
		return domain.ErrPositionUnknown
	case errors.Is(err, ErrPositionDetectionUnsupportedQueue):
		return ErrPositionDetectionUnsupportedQueue
	default:
		return nil
	}
}

func selectionFromSession(session champSelectSession) (domain.DetectedSelection, error) {
	if !isPositionDetectionQueueSupported(session.QueueID) {
		return domain.DetectedSelection{}, fmt.Errorf("%w: queueId %d", ErrPositionDetectionUnsupportedQueue, session.QueueID)
	}

	member, err := localPlayerFromSession(session)
	if err != nil {
		return domain.DetectedSelection{}, err
	}

	if member.ChampionID <= 0 {
		return domain.DetectedSelection{}, ErrChampionNotSelected
	}

	position, err := domain.PositionFromRaw(member.AssignedPosition)
	if err != nil {
		return domain.DetectedSelection{}, err
	}

	return domain.DetectedSelection{
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
