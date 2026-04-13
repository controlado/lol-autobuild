package lcu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func (c *Client) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx

	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply item set: %w", ErrNotConfigured)
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

	if err := validateApplySummonerSpellsRequest(req); err != nil {
		return err
	}

	var (
		lastErr                      error
		seenChampionSelectionChanged = false
		seenChampionNotSelected      = false
		seenSessionUnavailable       = false
		seenConnection               = false
	)

	for _, candidate := range c.connectionCandidates(ctx) {
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

		spell1ID, spell2ID := preserveFlashSlot(req.SpellIDs, member.Spell1ID, member.Spell2ID)
		if err := c.patchMySelectionSpells(ctx, info, spell1ID, spell2ID); err != nil {
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

func validateApplySummonerSpellsRequest(req ports.ApplySummonerSpellsRequest) error {
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

func preserveFlashSlot(spellIDs []int, currentSpell1ID int, currentSpell2ID int) (int, int) {
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

func (c *Client) patchMySelectionSpells(ctx context.Context, info lockfileInfo, spell1ID int, spell2ID int) error {
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

	applyLCUHeaders(req, info.Password)
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
