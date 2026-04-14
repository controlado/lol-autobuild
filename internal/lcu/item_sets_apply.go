package lcu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func (c *Client) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	itemIDs, err := validateItemSetApplyRequest(req)
	if err != nil {
		return err
	}

	var (
		managedSet       = newManagedItemSet(req, itemIDs)
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (shouldTerminate bool) {
			session, err := c.fetchChampSelectSession(ctx, info)
			if err != nil {
				attempt.observe(candidateLabel, ErrChampSelectUnavailable, err)
				return false
			}

			member, err := localPlayerFromSession(session)
			if err != nil {
				attempt.observe(candidateLabel, ErrChampSelectUnavailable, err)
				return false
			}

			if member.ChampionID <= 0 {
				err = fmt.Errorf("expected championId %d, got %d", req.ChampionID, member.ChampionID)
				attempt.observe(candidateLabel, ErrChampionNotSelected, err)
				return false
			}

			if member.ChampionID != req.ChampionID {
				err := fmt.Errorf("expected championId %d, got %d", req.ChampionID, member.ChampionID)
				attempt.observe(candidateLabel, ErrChampionSelectionChanged, err)
				return false
			}

			summoner, err := c.fetchCurrentSummoner(ctx, info)
			if err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			existing, err := c.fetchItemSets(ctx, info, summoner.SummonerID)
			if err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			payload, err := upsertManagedItemSet(existing, summoner.AccountID, managedSet)
			if err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			if err := c.putItemSets(ctx, info, summoner.SummonerID, payload); err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			return true
		}
	)

	if success, err := c.ForEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return err
	} else if success {
		return nil
	}

	return attempt.finish(
		ErrItemSetApplyFailed,
		ErrChampionSelectionChanged,
		ErrChampionNotSelected,
		ErrChampSelectUnavailable,
	)
}

func validateItemSetApplyRequest(req ports.ApplyItemSetRequest) ([]int, error) {
	if req.ChampionID <= 0 {
		return nil, fmt.Errorf("%w: championID must be > 0", ErrInvalidItemSetRequest)
	}
	if len(req.ItemIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one item ID is required", ErrInvalidItemSetRequest)
	}

	seen := make(map[int]struct{}, len(req.ItemIDs))
	itemIDs := make([]int, 0, len(req.ItemIDs))
	for _, itemID := range req.ItemIDs {
		if itemID <= 0 {
			return nil, fmt.Errorf("%w: item IDs must be > 0", ErrInvalidItemSetRequest)
		}
		if _, ok := seen[itemID]; ok {
			continue
		}

		seen[itemID] = struct{}{}
		itemIDs = append(itemIDs, itemID)
	}

	if len(itemIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one unique item ID is required", ErrInvalidItemSetRequest)
	}

	return itemIDs, nil
}

func upsertManagedItemSet(existing itemSetsPayload, fallbackAccountID int64, managed itemSet) (itemSetsPayload, error) {
	accountID := existing.AccountID
	if accountID == 0 && fallbackAccountID > 0 {
		accountID = uint64(fallbackAccountID)
	}
	if accountID == 0 {
		return itemSetsPayload{}, fmt.Errorf("%w: accountID must be > 0", ErrItemSetApplyFailed)
	}

	timestamp := existing.Timestamp
	if timestamp == 0 {
		timestamp = 1
	}

	managedRaw, err := json.Marshal(managed)
	if err != nil {
		return itemSetsPayload{}, fmt.Errorf("%w: encode managed set: %v", ErrItemSetApplyFailed, err)
	}

	managedUID := strings.TrimSpace(managed.UID)
	outSets := make([]json.RawMessage, 0, len(existing.ItemSets)+1)
	replaced := false
	for _, raw := range existing.ItemSets {
		if managedUID != "" && itemSetUIDFromRaw(raw) == managedUID {
			if !replaced {
				outSets = append(outSets, json.RawMessage(managedRaw))
				replaced = true
			}
			continue
		}
		outSets = append(outSets, append(json.RawMessage(nil), raw...))
	}

	if !replaced {
		outSets = append(outSets, json.RawMessage(managedRaw))
	}

	return itemSetsPayload{
		Timestamp: timestamp,
		AccountID: accountID,
		ItemSets:  outSets,
	}, nil
}

func newManagedItemSet(req ports.ApplyItemSetRequest, itemIDs []int) itemSet {
	items := make([]itemSetEntry, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		items = append(items, itemSetEntry{
			ID:    strconv.Itoa(itemID),
			Count: 1,
		})
	}

	return itemSet{
		UID:             managedItemSetUID(req.ChampionID, req.Role),
		Title:           managedItemSetTitle(req),
		Mode:            "any",
		Map:             "any",
		Type:            "custom",
		SortRank:        0,
		StartedFrom:     "blank",
		AssociatedChamp: []int{req.ChampionID},
		AssociatedMaps:  []int{11},
		Blocks: []itemSetBlock{
			{
				Type:  "Core",
				Items: items,
			},
		},
		PreferredItemSlot: []any{},
	}
}

func managedItemSetUID(championID int, role string) string {
	return fmt.Sprintf("lol-autobuild:%d:%s", championID, normalizeItemSetRole(role))
}

func managedItemSetTitle(req ports.ApplyItemSetRequest) string {
	title := fmt.Sprintf("AutoBuild %d %s", req.ChampionID, normalizeItemSetRole(req.Role))
	patch := strings.TrimSpace(req.Patch)
	if patch != "" {
		title += " " + patch
	}

	return title
}

func normalizeItemSetRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "middle":
		return "mid"
	case "bot":
		return "adc"
	case "sup":
		return "support"
	default:
		normalized := strings.ToLower(strings.TrimSpace(role))
		if normalized == "" {
			return "unknown"
		}
		return strings.ReplaceAll(normalized, " ", "-")
	}
}

func itemSetUIDFromRaw(raw json.RawMessage) string {
	var payload itemSetUID
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.UID)
}
