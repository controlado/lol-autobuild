package lcu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
)

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

	managedSet := newManagedItemSet(req, itemIDs)

	var (
		lastErr                      error
		seenChampionSelectionChanged = false
		seenChampionNotSelected      = false
		seenSessionUnavailable       = false
		seenConnection               = false
	)

	for _, candidate := range c.candidates(ctx) {
		info, err := candidate.resolve()
		if err != nil {
			if !errors.Is(err, ErrLockfileNotFound) {
				seenConnection = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}
		seenConnection = true

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		member, err := localPlayerFromSession(session)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		if member.ChampionID <= 0 {
			seenChampionNotSelected = true
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), ErrChampionNotSelected)
			continue
		}

		if member.ChampionID != req.ChampionID {
			seenChampionSelectionChanged = true
			lastErr = fmt.Errorf("candidate %q: %w: expected championId %d, got %d", candidate.label(), ErrChampionSelectionChanged, req.ChampionID, member.ChampionID)
			continue
		}

		summoner, err := c.fetchCurrentSummoner(ctx, info)
		if err != nil {
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		existing, err := c.fetchItemSets(ctx, info, summoner.SummonerID)
		if err != nil {
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		payload, err := upsertManagedItemSet(existing, summoner.AccountID, managedSet)
		if err != nil {
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		if err := c.putItemSets(ctx, info, summoner.SummonerID, payload); err != nil {
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		return nil
	}

	if seenChampionSelectionChanged {
		return withLastCandidateError(ErrChampionSelectionChanged, lastErr)
	}
	if seenChampionNotSelected {
		return withLastCandidateError(ErrChampionNotSelected, lastErr)
	}
	if seenSessionUnavailable {
		return withLastCandidateError(ErrChampSelectUnavailable, lastErr)
	}
	if !seenConnection {
		return ErrLockfileNotFound
	}

	return withLastCandidateError(ErrItemSetApplyFailed, lastErr)
}

func (c *Client) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx

	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply rune page: %w", ErrNotConfigured)
}

func (c *Client) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	if err := validateSpellApplyRequest(req); err != nil {
		return err
	}

	var (
		lastErr                      error
		seenChampionSelectionChanged = false
		seenChampionNotSelected      = false
		seenSessionUnavailable       = false
		seenConnection               = false
	)

	for _, candidate := range c.candidates(ctx) {
		info, err := candidate.resolve()
		if err != nil {
			if !errors.Is(err, ErrLockfileNotFound) {
				seenConnection = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}
		seenConnection = true

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		member, err := localPlayerFromSession(session)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		if member.ChampionID <= 0 {
			seenChampionNotSelected = true
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), ErrChampionNotSelected)
			continue
		}

		if member.ChampionID != req.ChampionID {
			seenChampionSelectionChanged = true
			lastErr = fmt.Errorf("candidate %q: %w: expected championId %d, got %d", candidate.label(), ErrChampionSelectionChanged, req.ChampionID, member.ChampionID)
			continue
		}

		spell1ID, spell2ID := keepFlashSlot(req.SpellIDs, member.Spell1ID, member.Spell2ID)
		if err := c.patchSelectionSpells(ctx, info, spell1ID, spell2ID); err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("candidate %q: %w", candidate.label(), err)
			continue
		}

		return nil
	}

	if seenChampionSelectionChanged {
		return withLastCandidateError(ErrChampionSelectionChanged, lastErr)
	}
	if seenChampionNotSelected {
		return withLastCandidateError(ErrChampionNotSelected, lastErr)
	}
	if seenSessionUnavailable {
		return withLastCandidateError(ErrChampSelectUnavailable, lastErr)
	}
	if !seenConnection {
		return ErrLockfileNotFound
	}

	return withLastCandidateError(ErrSummonerSpellsApplyFailed, lastErr)
}

func validateSpellApplyRequest(req ports.ApplySummonerSpellsRequest) error {
	if req.ChampionID <= 0 {
		return fmt.Errorf("%w: championID must be > 0", ErrInvalidSummonerSpellsRequest)
	}
	if len(req.SpellIDs) != 2 {
		return fmt.Errorf("%w: exactly 2 spell IDs are required", ErrInvalidSummonerSpellsRequest)
	}
	if req.SpellIDs[0] <= 0 || req.SpellIDs[1] <= 0 {
		return fmt.Errorf("%w: spell IDs must be > 0", ErrInvalidSummonerSpellsRequest)
	}
	if req.SpellIDs[0] == req.SpellIDs[1] {
		return fmt.Errorf("%w: spell IDs must be distinct", ErrInvalidSummonerSpellsRequest)
	}
	return nil
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

func keepFlashSlot(spellIDs []int, currentSpell1ID int, currentSpell2ID int) (int, int) {
	var (
		spell1ID = spellIDs[0]
		spell2ID = spellIDs[1]
	)

	// se não possui flash nas spells requisitadas
	containsFlash := spell1ID == 4 || spell2ID == 4
	if !containsFlash {
		return spell1ID, spell2ID
	}

	// otherSpellID == outra spell fora o flash
	// se a primeira spell for flash, configura
	// otherSpellID para a segunda spell
	otherSpellID := spell1ID
	if spell1ID == 4 {
		otherSpellID = spell2ID
	}

	switch {
	case currentSpell1ID == 4:
		return 4, otherSpellID
	case currentSpell2ID == 4:
		return otherSpellID, 4
	default:
		return spell1ID, spell2ID
	}
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

func itemSetUIDFromRaw(raw json.RawMessage) string {
	var payload itemSetUID
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.UID)
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
