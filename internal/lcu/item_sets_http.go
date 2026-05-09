package lcu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Client) fetchCurrentSummoner(ctx context.Context, info connectionInfo) (currentSummonerInfo, error) {
	summonerInfo, err := doJSON[currentSummonerInfo](ctx, c, info, http.MethodGet, currentSummonerPath, nil)
	if err != nil {
		return currentSummonerInfo{}, fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}

	if summonerInfo.SummonerID <= 0 {
		summonerInfo.SummonerID = summonerInfo.ID
	}
	if summonerInfo.SummonerID <= 0 || summonerInfo.AccountID <= 0 {
		return currentSummonerInfo{}, fmt.Errorf("%w: missing summoner/account ids", ErrItemSetApplyFailed)
	}

	return summonerInfo, nil
}

func (c *Client) fetchItemSets(ctx context.Context, info connectionInfo, summonerID int64) (itemSetsPayload, error) {
	endpoint := fmt.Sprintf(itemSetsPathFormat, summonerID)
	itemSetsResult, err := doJSON[itemSetsPayload](ctx, c, info, http.MethodGet, endpoint, nil)
	if err != nil {
		return itemSetsPayload{}, fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}

	if itemSetsResult.ItemSets == nil {
		itemSetsResult.ItemSets = []json.RawMessage{}
	}

	return itemSetsResult, nil
}

func (c *Client) putItemSets(ctx context.Context, info connectionInfo, summonerID int64, payload itemSetsPayload) error {
	endpoint := fmt.Sprintf(itemSetsPathFormat, summonerID)
	if err := doRequest(ctx, c, info, http.MethodPut, endpoint, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrItemSetApplyFailed, err)
	}
	return nil
}
