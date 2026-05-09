package lcu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
)

func (c *Client) fetchChampSelectSession(ctx context.Context, info connectionInfo) (champSelectSession, error) {
	session, err := doJSON[champSelectSession](ctx, c, info, http.MethodGet, champSelectSessionPath, nil)
	if err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return champSelectSession{}, ErrChampSelectUnavailable
		}
		return champSelectSession{}, fmt.Errorf("%w: %v", ErrChampSelectUnavailable, err)
	}
	return session, nil
}

func (c *Client) fetchChampSelectSessionEventData(ctx context.Context, info connectionInfo) (json.RawMessage, error) {
	raw, err := doJSON[json.RawMessage](ctx, c, info, http.MethodGet, champSelectSessionPath, nil)
	if err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return nil, ErrChampSelectUnavailable
		}
		return nil, fmt.Errorf("%w: %v", ErrChampSelectUnavailable, err)
	}
	return slices.Clone(raw), nil
}

func (c *Client) patchSelectionSpells(ctx context.Context, info connectionInfo, spell1ID int, spell2ID int) error {
	var (
		endpoint = champSelectMySelectionPath
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
