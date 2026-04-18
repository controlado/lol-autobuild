package lcu

import (
	"context"
	"errors"
	"fmt"

	"github.com/controlado/lol-autobuild/internal/ports"
)

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
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (success bool) {
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

			spell1ID, spell2ID := keepFlashSlot(req.SpellIDs, member.Spell1ID, member.Spell2ID)
			if err := c.patchSelectionSpells(ctx, info, spell1ID, spell2ID); err != nil {
				if errors.Is(err, ErrChampSelectUnavailable) {
					attempt.observe(candidateLabel, ErrChampSelectUnavailable, err)
				} else {
					attempt.observe(candidateLabel, nil, err)
				}
				return false
			}

			return true
		}
	)

	if success, err := c.forEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return err
	} else if success {
		return nil
	}

	return attempt.finish(
		ErrSummonerSpellsApplyFailed,
		ErrChampionSelectionChanged,
		ErrChampionNotSelected,
		ErrChampSelectUnavailable,
	)
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
