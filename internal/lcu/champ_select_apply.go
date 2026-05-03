package lcu

import (
	"context"
	"fmt"
)

type localPlayerSelectionValidation struct {
	member  champSelectPlayerSelection
	baseErr error
	err     error
}

func (c *Client) validatedLocalPlayerSelection(ctx context.Context, info connectionInfo, championID int) localPlayerSelectionValidation {
	session, err := c.fetchChampSelectSession(ctx, info)
	if err != nil {
		return localPlayerSelectionValidation{baseErr: ErrChampSelectUnavailable, err: err}
	}

	member, err := localPlayerFromSession(session)
	if err != nil {
		return localPlayerSelectionValidation{baseErr: ErrChampSelectUnavailable, err: err}
	}

	if member.ChampionID <= 0 {
		err = fmt.Errorf("expected championId %d, got %d", championID, member.ChampionID)
		return localPlayerSelectionValidation{baseErr: ErrChampionNotSelected, err: err}
	}

	if member.ChampionID != championID {
		err := fmt.Errorf("expected championId %d, got %d", championID, member.ChampionID)
		return localPlayerSelectionValidation{baseErr: ErrChampionSelectionChanged, err: err}
	}

	return localPlayerSelectionValidation{member: member}
}
