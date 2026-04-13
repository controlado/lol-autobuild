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

func (c *Client) fetchCurrentSummoner(ctx context.Context, info connectionInfo) (currentSummonerInfo, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-summoner/v1/current-summoner", info.Protocol, info.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return currentSummonerInfo{}, fmt.Errorf("%w: build request: %v", ErrItemSetApplyFailed, err)
	}

	applyHeaders(req, info.Password)

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return currentSummonerInfo{}, fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var out currentSummonerInfo
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return currentSummonerInfo{}, fmt.Errorf("%w: decode response: %v", ErrItemSetApplyFailed, err)
		}
		if out.SummonerID <= 0 {
			out.SummonerID = out.ID
		}
		if out.SummonerID <= 0 || out.AccountID <= 0 {
			return currentSummonerInfo{}, fmt.Errorf("%w: missing summoner/account ids", ErrItemSetApplyFailed)
		}
		return out, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return currentSummonerInfo{}, fmt.Errorf("%w: status %d", ErrItemSetApplyFailed, resp.StatusCode)
		}
		return currentSummonerInfo{}, fmt.Errorf("%w: status %d: %s", ErrItemSetApplyFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c *Client) fetchItemSets(ctx context.Context, info connectionInfo, summonerID int64) (itemSetsPayload, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-item-sets/v1/item-sets/%d/sets", info.Protocol, info.Port, summonerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return itemSetsPayload{}, fmt.Errorf("%w: build request: %v", ErrItemSetApplyFailed, err)
	}

	applyHeaders(req, info.Password)

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return itemSetsPayload{}, fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var out itemSetsPayload
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return itemSetsPayload{}, fmt.Errorf("%w: decode response: %v", ErrItemSetApplyFailed, err)
		}
		if out.ItemSets == nil {
			out.ItemSets = []json.RawMessage{}
		}
		return out, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return itemSetsPayload{}, fmt.Errorf("%w: status %d", ErrItemSetApplyFailed, resp.StatusCode)
		}
		return itemSetsPayload{}, fmt.Errorf("%w: status %d: %s", ErrItemSetApplyFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c *Client) putItemSets(ctx context.Context, info connectionInfo, summonerID int64, payload itemSetsPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: encode payload: %v", ErrItemSetApplyFailed, err)
	}

	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-item-sets/v1/item-sets/%d/sets", info.Protocol, info.Port, summonerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrItemSetApplyFailed, err)
	}

	applyHeaders(req, info.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return fmt.Errorf("%w: status %d", ErrItemSetApplyFailed, resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d: %s", ErrItemSetApplyFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
