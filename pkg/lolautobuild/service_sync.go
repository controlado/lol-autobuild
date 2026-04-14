package lolautobuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/controlado/lol-autobuild/internal/ports"
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

	stageSpecs := itemStageSpecsForRole(selection.Role)

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
			Role:       selection.Role,
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

func itemStageSpecsForRole(role string) []itemStageSpec {
	firstItemType := itemTypeLegendaries
	if isSupportRole(role) {
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

func isSupportRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "support", "sup", "4":
		return true
	default:
		return false
	}
}

func filterAndLimitItemStats(in []ports.ItemStat, minOccurrence, topItems int) []ports.ItemStat {
	out := make([]ports.ItemStat, 0, len(in))
	for _, stat := range in {
		if stat.Occurrence >= minOccurrence {
			out = append(out, stat)
		}
	}

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
