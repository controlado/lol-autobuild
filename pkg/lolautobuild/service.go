package lolautobuild

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type RecommendationPolicy struct {
	MinOccurrence int
	TopItems      int
	TopSpells     int
}

type ServiceDeps struct {
	Coachless   ports.CoachlessClient
	Tokens      ports.TokenProvider
	LCU         ports.LCUClient
	Recommender ports.RecommendationEngine
	Policy      RecommendationPolicy
}

type syncService struct {
	deps ServiceDeps
}

func NewService(deps ServiceDeps) (Service, error) {
	if deps.Coachless == nil {
		return nil, errors.New("coachless client is required")
	}
	if deps.Tokens == nil {
		return nil, errors.New("token provider is required")
	}
	if deps.LCU == nil {
		return nil, errors.New("lcu client is required")
	}
	if deps.Recommender == nil {
		return nil, errors.New("recommendation engine is required")
	}
	if deps.Policy.MinOccurrence < 0 {
		return nil, errors.New("policy.min_occurrence must be >= 0")
	}
	if deps.Policy.TopItems <= 0 {
		deps.Policy.TopItems = 6
	}
	if deps.Policy.TopSpells <= 0 {
		deps.Policy.TopSpells = 2
	}

	return &syncService{deps: deps}, nil
}

func (s *syncService) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	selection, err := s.deps.LCU.DetectSelection(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("detect local selection: %w", err)
	}

	accessToken, err := s.deps.Tokens.AccessToken(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get access token: %w", err)
	}

	patches, err := s.deps.Coachless.GetPatches(ctx, accessToken)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get patches: %w", err)
	}

	patchFilter, patchLabel, err := resolvePatch(req.Patch, patches)
	if err != nil {
		return SyncResult{}, err
	}

	roleCode := roleToCode(selection.Role)

	filters := ports.CommonFilters{
		Patch:       patchFilter,
		ChampionIDs: []int{selection.ChampionID},
		LeagueTiers: []int{5, 6, 7},
		Role:        roleCode,
	}

	keystoneStats, err := s.deps.Coachless.GetKeystoneData(ctx, accessToken, ports.KeystoneRequest{CommonFilters: filters})
	if err != nil {
		return SyncResult{}, fmt.Errorf("get keystone data: %w", err)
	}

	spellStats, err := s.deps.Coachless.GetSummonerSpellStats(ctx, accessToken, ports.SummonerSpellStatsRequest{
		CommonFilters: filters,
		PairedSpell:   nil,
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("get summoner spell stats: %w", err)
	}

	itemStats, err := s.deps.Coachless.GetItemStats(ctx, accessToken, ports.ItemStatsRequest{
		CommonFilters:         filters,
		ItemType:              6,
		LoadFirstEpicPurchase: false,
		IncludeSupportItems:   false,
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("get item stats: %w", err)
	}

	rec := s.deps.Recommender.Recommend(ports.RecommendationInput{
		KeystoneStats: keystoneStats,
		SpellStats:    spellStats,
		ItemStats:     itemStats,
		MinOccurrence: s.deps.Policy.MinOccurrence,
		TopItems:      s.deps.Policy.TopItems,
		TopSpells:     s.deps.Policy.TopSpells,
	})

	result := SyncResult{
		DetectedChampionID: selection.ChampionID,
		DetectedRole:       selection.Role,
		DetectedQueueID:    selection.QueueID,
		Warnings:           append([]string{}, rec.Warnings...),
	}
	result.Warnings = append(result.Warnings, fmt.Sprintf("selected patch: %s", patchLabel))
	if selection.IsAutofilled {
		result.Warnings = append(result.Warnings, "autofill detected in current champ select")
	}

	if req.DryRun {
		result.Warnings = append(result.Warnings, "dry-run enabled: no LCU changes were applied")
		return result, nil
	}

	if req.ApplyRunes {
		if rec.Keystone == nil {
			result.Warnings = append(result.Warnings, "apply runes requested but no keystone recommendation was available")
		} else if err := s.deps.LCU.ApplyRunePage(ctx, ports.ApplyRunePageRequest{
			ChampionID: selection.ChampionID,
			Role:       selection.Role,
			KeystoneID: rec.Keystone.Rune,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply rune page: "+err.Error())
		} else {
			result.RunePageApplied = true
		}
	}

	if req.ApplySpells {
		spellIDs := make([]int, 0, len(rec.SummonerSpells))
		for _, stat := range rec.SummonerSpells {
			spellIDs = append(spellIDs, stat.SummonerSpell)
		}

		if len(spellIDs) == 0 {
			result.Warnings = append(result.Warnings, "apply spells requested but no spell recommendation was available")
		} else if err := s.deps.LCU.ApplySummonerSpells(ctx, ports.ApplySummonerSpellsRequest{
			ChampionID: selection.ChampionID,
			Role:       selection.Role,
			SpellIDs:   spellIDs,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply summoner spells: "+err.Error())
		} else {
			result.SpellsApplied = true
		}
	}

	if req.ApplyItems {
		itemIDs := make([]int, 0, len(rec.Items))
		for _, stat := range rec.Items {
			itemIDs = append(itemIDs, stat.ItemID)
		}

		if len(itemIDs) == 0 {
			result.Warnings = append(result.Warnings, "apply items requested but no item recommendation was available")
		} else if err := s.deps.LCU.ApplyItemSet(ctx, ports.ApplyItemSetRequest{
			ChampionID: selection.ChampionID,
			Role:       selection.Role,
			Patch:      patchLabel,
			ItemIDs:    itemIDs,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply item set: "+err.Error())
		} else {
			result.ItemSetApplied = true
		}
	}

	return result, nil
}

func resolvePatch(rawPatch string, patches []ports.PatchInfo) (ports.PatchFilter, string, error) {
	if len(patches) == 0 {
		return ports.PatchFilter{}, "", errors.New("no patch data available")
	}

	selected := patches[len(patches)-1]
	if strings.TrimSpace(rawPatch) != "" {
		wanted := strings.TrimSpace(rawPatch)
		found := false
		for _, p := range patches {
			if p.Label == wanted {
				selected = p
				found = true
				break
			}
		}
		if !found {
			return ports.PatchFilter{}, "", fmt.Errorf("requested patch %q not found", rawPatch)
		}
	}

	return ports.PatchFilter{
		Major:          selected.Major,
		Patch:          selected.Patch,
		PatchAdditions: 2,
	}, selected.Label, nil
}

func roleToCode(role string) int {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "top":
		return 0
	case "jungle":
		return 1
	case "mid", "middle":
		return 2
	case "adc", "bot":
		return 3
	case "support", "sup":
		return 4
	default:
		if v, err := strconv.Atoi(role); err == nil {
			return v
		}
		return 0
	}
}
