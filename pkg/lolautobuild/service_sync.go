package lolautobuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/position"
	"golang.org/x/sync/errgroup"
)

func (s *syncService) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	selection, err := s.deps.LCU.DetectSelection(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("detect local selection: %w", err)
	}

	accessToken, err := s.deps.Tokens.AccessToken(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get access token: %w", err)
	}

	tokenClaims, err := s.deps.Tokens.Claims(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get token claims: %w", err)
	}

	patches, err := s.deps.Coachless.GetPatches(ctx, accessToken)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get patches: %w", err)
	}

	patchFilter, patchLabel, patchWarnings, err := resolvePatch(
		req.Patch,
		req.PatchAdditionsMode,
		req.PatchAdditions,
		patches,
		tokenClaims.IsSubscribed(),
	)
	if err != nil {
		return SyncResult{}, err
	}

	leagueTiers, err := resolveLeagueTierPreset(req.LeagueTierPreset)
	if err != nil {
		return SyncResult{}, err
	}

	filters := ports.CommonFilters{
		Patch:       patchFilter,
		ChampionIDs: []int{selection.ChampionID},
		LeagueTiers: leagueTiers,
		Role:        selection.Position.Code(),
	}

	stageSpecs := itemStageSpecsForPosition(selection.Position)

	var keystoneStats []ports.KeystoneStat
	var spellStats []ports.SummonerSpellStat
	stageStats := make([][]ports.ItemStat, len(stageSpecs))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	g.Go(func() error {
		var err error
		keystoneStats, err = s.deps.Coachless.GetKeystoneData(gctx, accessToken, ports.KeystoneRequest{CommonFilters: filters})
		if err != nil {
			return fmt.Errorf("get keystone data: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		spellStats, err = s.deps.Coachless.GetSummonerSpellStats(gctx, accessToken, ports.SummonerSpellStatsRequest{
			CommonFilters: filters,
			PairedSpell:   nil,
		})
		if err != nil {
			return fmt.Errorf("get summoner spell stats: %w", err)
		}
		return nil
	})

	for idx, stage := range stageSpecs {
		g.Go(func() error {
			stats, err := s.deps.Coachless.GetItemStats(gctx, accessToken, ports.ItemStatsRequest{
				CommonFilters:         filters,
				ItemSlots:             stage.ItemSlots,
				ItemType:              stage.ItemType,
				LoadFirstEpicPurchase: false,
				IncludeSupportItems:   stage.IncludeSupportItems,
			})
			if err != nil {
				return fmt.Errorf("get item stats for %s: %w", stage.Type, err)
			}

			stageStats[idx] = stats
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return SyncResult{}, err
	}

	itemStats := make([]ports.ItemStat, 0)
	for _, stats := range stageStats {
		itemStats = append(itemStats, stats...)
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
		DetectedPosition:   selection.Position.String(),
		DetectedQueueID:    selection.QueueID,
		Warnings:           append([]string{}, rec.Warnings...),
	}
	result.Warnings = append(result.Warnings, selectedPatchWarning(patchLabel, patchFilter.PatchAdditions))
	result.Warnings = append(result.Warnings, patchWarnings...)
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
			Position:   selection.Position,
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
			Position:   selection.Position,
			SpellIDs:   spellIDs,
			KeepFlash:  req.KeepFlash,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply summoner spells: "+err.Error())
		} else {
			result.SpellsApplied = true
		}
	}

	if req.ApplyItems {
		blocks := make([]ports.ApplyItemSetBlock, 0, len(stageSpecs))
		hasAnyItems := false
		for idx, stage := range stageSpecs {
			filtered := filterAndLimitItemStats(stageStats[idx], s.deps.Policy.MinOccurrence, s.deps.Policy.TopItems)
			blockItemIDs := make([]int, 0, len(filtered))
			for _, stat := range filtered {
				blockItemIDs = append(blockItemIDs, stat.ItemID)
			}
			if len(blockItemIDs) > 0 {
				hasAnyItems = true
			}

			blocks = append(blocks, ports.ApplyItemSetBlock{
				Type:    stage.Type,
				ItemIDs: blockItemIDs,
			})
		}

		if !hasAnyItems {
			result.Warnings = append(result.Warnings, "apply items requested but no item recommendation was available")
		} else if err := s.deps.LCU.ApplyItemSet(ctx, ports.ApplyItemSetRequest{
			ChampionID: selection.ChampionID,
			Position:   selection.Position,
			Patch:      patchLabel,
			Blocks:     blocks,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply item set: "+err.Error())
		} else {
			result.ItemSetApplied = true
		}
	}

	return result, nil
}

const (
	itemTypeLegendaries = 1
	itemTypeBoots       = 2
	itemTypeSupport     = 3
	itemTypeStarter     = 6
)

type itemStageSpec struct {
	Type                string
	ItemType            int
	ItemSlots           []int
	IncludeSupportItems bool
}

func itemStageSpecsForPosition(p position.Position) []itemStageSpec {
	firstItemType := itemTypeLegendaries
	if p.IsSupport() {
		firstItemType = itemTypeSupport
	}

	return []itemStageSpec{
		{
			Type:     "Starter",
			ItemType: itemTypeStarter,
		},
		{
			Type:                "1st Item",
			ItemType:            firstItemType,
			ItemSlots:           []int{1},
			IncludeSupportItems: true,
		},
		{
			Type:      "2nd Item",
			ItemType:  itemTypeLegendaries,
			ItemSlots: []int{2},
		},
		{
			Type:     "Boots",
			ItemType: itemTypeBoots,
		},
		{
			Type:      "3rd Item",
			ItemType:  itemTypeLegendaries,
			ItemSlots: []int{3},
		},
		{
			Type:      "4th+ Item",
			ItemType:  itemTypeLegendaries,
			ItemSlots: []int{4, 5, 6},
		},
	}
}

func filterAndLimitItemStats(in []ports.ItemStat, minOccurrence, topItems int) []ports.ItemStat {
	out := itemStatsPassingOccurrenceFilter(in, minOccurrence)

	sort.Slice(out, func(i, j int) bool {
		if out[i].WPAOverall == out[j].WPAOverall {
			return out[i].Occurrence > out[j].Occurrence
		}
		return out[i].WPAOverall > out[j].WPAOverall
	})

	if topItems > 0 && len(out) > topItems {
		out = out[:topItems]
	}

	return out
}

func itemStatsPassingOccurrenceFilter(in []ports.ItemStat, minOccurrence int) []ports.ItemStat {
	filtered := make([]ports.ItemStat, 0, len(in))
	for _, stat := range in {
		if stat.Occurrence >= minOccurrence {
			filtered = append(filtered, stat)
		}
	}

	if len(filtered) > 0 || len(in) == 0 {
		return filtered
	}

	return append([]ports.ItemStat{}, in...)
}

func resolvePatch(rawPatch string, rawPatchAdditionsMode string, requestedPatchAdditions int, patches []ports.PatchInfo, subscribed bool) (ports.PatchFilter, string, []string, error) {
	if len(patches) == 0 {
		return ports.PatchFilter{}, "", nil, errors.New("no patch data available")
	}

	patchAdditionsMode := strings.TrimSpace(rawPatchAdditionsMode)
	if patchAdditionsMode == "" {
		patchAdditionsMode = PatchAdditionsModeAuto
	}
	if patchAdditionsMode != PatchAdditionsModeAuto && patchAdditionsMode != PatchAdditionsModeManual {
		return ports.PatchFilter{}, "", nil, fmt.Errorf("patch additions mode %q is invalid", rawPatchAdditionsMode)
	}
	if requestedPatchAdditions < 0 || requestedPatchAdditions > PatchAdditionsMax {
		return ports.PatchFilter{}, "", nil, fmt.Errorf("patch additions %d must be between 0 and %d", requestedPatchAdditions, PatchAdditionsMax)
	}

	selectedIndex := len(patches) - 1
	if strings.TrimSpace(rawPatch) != "" {
		wanted := strings.TrimSpace(rawPatch)
		found := false
		for idx, p := range patches {
			if p.Label == wanted {
				selectedIndex = idx
				found = true
				break
			}
		}
		if !found {
			return ports.PatchFilter{}, "", nil, fmt.Errorf("requested patch %q not found", rawPatch)
		}
	} else if !subscribed && len(patches) > 1 {
		selectedIndex = len(patches) - 2
	}

	if !subscribed && selectedIndex == len(patches)-1 && len(patches) > 1 {
		return ports.PatchFilter{}, "", nil, fmt.Errorf("requested patch %q requires Coachless Premium", patches[selectedIndex].Label)
	}

	selected := patches[selectedIndex]
	patchAdditions, warnings, err := resolvePatchAdditions(patchAdditionsMode, requestedPatchAdditions, selectedIndex, subscribed)
	if err != nil {
		return ports.PatchFilter{}, "", nil, err
	}

	return ports.PatchFilter{
		Major:          selected.Major,
		Patch:          selected.Patch,
		PatchAdditions: patchAdditions,
	}, selected.Label, warnings, nil
}

func resolvePatchAdditions(mode string, requestedPatchAdditions int, selectedPatchIndex int, subscribed bool) (int, []string, error) {
	if mode == PatchAdditionsModeAuto {
		if !subscribed {
			return 0, nil, nil
		}
		return min(PatchAdditionsDefault, selectedPatchIndex), nil, nil
	}

	if requestedPatchAdditions > 0 && !subscribed {
		return 0, nil, errors.New("requested patch additions require Coachless Premium")
	}

	maxAvailable := min(PatchAdditionsMax, selectedPatchIndex)
	if requestedPatchAdditions <= maxAvailable {
		return requestedPatchAdditions, nil, nil
	}

	warnings := []string{
		fmt.Sprintf("patch additions reduced from %d to %d because only %d previous patches are available", requestedPatchAdditions, maxAvailable, maxAvailable),
	}
	return maxAvailable, warnings, nil
}

func selectedPatchWarning(patchLabel string, patchAdditions int) string {
	switch {
	case patchAdditions <= 0:
		return fmt.Sprintf("selected patch: %s", patchLabel)
	case patchAdditions == 1:
		return fmt.Sprintf("selected patch: %s (+1 previous patch)", patchLabel)
	default:
		return fmt.Sprintf("selected patch: %s (+%d previous patches)", patchLabel, patchAdditions)
	}
}

func resolveLeagueTierPreset(rawPreset string) ([]int, error) {
	preset := strings.TrimSpace(rawPreset)
	if preset == "" {
		preset = LeagueTierPresetDefault
	}

	switch preset {
	case LeagueTierPresetGoldPlus:
		return []int{3, 4, 5, 6, 7}, nil
	case LeagueTierPresetPlatinumPlus:
		return []int{4, 5, 6, 7}, nil
	case LeagueTierPresetEmeraldPlus:
		return []int{5, 6, 7}, nil
	case LeagueTierPresetDiamondPlus:
		return []int{6, 7}, nil
	case LeagueTierPresetMasterPlus:
		return []int{7}, nil
	default:
		return nil, fmt.Errorf("league tier preset %q is invalid", rawPreset)
	}
}
