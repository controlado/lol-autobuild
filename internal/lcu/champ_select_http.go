package lcu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

func (c *Client) fetchChampSelectSession(ctx context.Context, info connectionInfo) (champSelectSession, error) {
	session, err := doJSON[champSelectSession](ctx, c, info, http.MethodGet, "/lol-champ-select/v1/session", nil)
	if err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return champSelectSession{}, ErrChampSelectUnavailable
		}
		return champSelectSession{}, fmt.Errorf("%w: %v", ErrChampSelectUnavailable, err)
	}
	return session, nil
}

func (c *Client) patchSelectionSpells(ctx context.Context, info connectionInfo, spell1ID int, spell2ID int) error {
	var (
		endpoint = "/lol-champ-select/v1/session/my-selection"
		payload  = champSelectMySelectionPatch{Spell1ID: spell1ID, Spell2ID: spell2ID}
	)
	if err := doRequest(ctx, c, info, http.MethodPatch, endpoint, payload); err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return ErrChampSelectUnavailable
		}
		return fmt.Errorf("%w: %v", ErrSummonerSpellsApplyFailed, err)
	}
	return nil
}
