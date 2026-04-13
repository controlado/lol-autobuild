package lcu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) fetchChampSelectSession(ctx context.Context, info connectionInfo) (champSelectSession, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-champ-select/v1/session", info.Protocol, info.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return champSelectSession{}, fmt.Errorf("%w: build request: %v", ErrChampSelectUnavailable, err)
	}

	applyHeaders(req, info.Password)

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return champSelectSession{}, fmt.Errorf("%w: %v", ErrChampSelectUnavailable, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var session champSelectSession
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			return champSelectSession{}, fmt.Errorf("%w: decode response: %v", ErrChampSelectUnavailable, err)
		}
		return session, nil
	case http.StatusNotFound:
		return champSelectSession{}, ErrChampSelectUnavailable
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return champSelectSession{}, fmt.Errorf("%w: status %d", ErrChampSelectUnavailable, resp.StatusCode)
		}
		return champSelectSession{}, fmt.Errorf("%w: status %d: %s", ErrChampSelectUnavailable, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c *Client) patchSelectionSpells(ctx context.Context, info connectionInfo, spell1ID int, spell2ID int) error {
	payload, err := json.Marshal(champSelectMySelectionPatch{
		Spell1ID: spell1ID,
		Spell2ID: spell2ID,
	})
	if err != nil {
		return fmt.Errorf("%w: encode payload: %v", ErrSummonerSpellsApplyFailed, err)
	}

	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-champ-select/v1/session/my-selection", info.Protocol, info.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrSummonerSpellsApplyFailed, err)
	}

	applyHeaders(req, info.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSummonerSpellsApplyFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrChampSelectUnavailable
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return fmt.Errorf("%w: status %d", ErrSummonerSpellsApplyFailed, resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d: %s", ErrSummonerSpellsApplyFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
