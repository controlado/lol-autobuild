package lcu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type championNamePayload struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (c *Client) fetchChampionSummaryNames(ctx context.Context, info connectionInfo) (map[int]string, error) {
	raw, err := doJSON[json.RawMessage](ctx, c, info, http.MethodGet, championSummaryURI, nil)
	if err != nil {
		return nil, err
	}

	names, err := championSummaryNamesFromJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("decode champion summary: %w", err)
	}
	return names, nil
}

func (c *Client) fetchChampionName(ctx context.Context, info connectionInfo, championID int) (string, error) {
	path := fmt.Sprintf(championDetailsURIFormat, championID)
	payload, err := doJSON[championNamePayload](ctx, c, info, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return "", fmt.Errorf("champion %d name is empty", championID)
	}
	return name, nil
}

func championSummaryNamesFromJSON(raw json.RawMessage) (map[int]string, error) {
	var entries []championNamePayload
	if err := json.Unmarshal(raw, &entries); err == nil {
		return championNamesFromPayload(entries), nil
	}

	var wrapped struct {
		Champions []championNamePayload `json:"champions"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}

	return championNamesFromPayload(wrapped.Champions), nil
}

func championNamesFromPayload(entries []championNamePayload) map[int]string {
	names := make(map[int]string, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if entry.ID > 0 && name != "" {
			names[entry.ID] = name
		}
	}
	return names
}
