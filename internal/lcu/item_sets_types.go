package lcu

import "encoding/json"

type currentSummonerInfo struct {
	SummonerID int64 `json:"summonerId"`
	AccountID  int64 `json:"accountId"`
	ID         int64 `json:"id"`
}

type itemSetsPayload struct {
	Timestamp uint64            `json:"timestamp"`
	AccountID uint64            `json:"accountId"`
	ItemSets  []json.RawMessage `json:"itemSets"`
}

type itemSet struct {
	UID               string         `json:"uid"`
	Title             string         `json:"title"`
	Mode              string         `json:"mode"`
	Map               string         `json:"map"`
	Type              string         `json:"type"`
	SortRank          int            `json:"sortrank"`
	StartedFrom       string         `json:"startedFrom"`
	AssociatedChamp   []int          `json:"associatedChampions"`
	AssociatedMaps    []int          `json:"associatedMaps"`
	Blocks            []itemSetBlock `json:"blocks"`
	PreferredItemSlot []any          `json:"preferredItemSlots"`
}

type itemSetBlock struct {
	Type  string         `json:"type"`
	Items []itemSetEntry `json:"items"`
}

type itemSetEntry struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type itemSetUID struct {
	UID string `json:"uid"`
}
