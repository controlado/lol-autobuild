package recommend

import (
	"slices"
	"sort"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
	"github.com/controlado/lol-autobuild/internal/autobuild/runes"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Recommend(input domain.RecommendationInput) domain.Recommendation {
	out := domain.Recommendation{}

	keystones := filterKeystones(input.KeystoneStats, input.MinOccurrence)
	if len(keystones) > 0 {
		out.Keystone = &keystones[0]
	} else {
		out.Warnings = append(out.Warnings, "no keystone passed occurrence threshold")
	}

	spells := filterSpells(input.SpellStats, input.MinOccurrence)
	if input.TopSpells > 0 && len(spells) > input.TopSpells {
		spells = spells[:input.TopSpells]
	}
	out.SummonerSpells = spells
	if len(spells) == 0 {
		out.Warnings = append(out.Warnings, "no summoner spell passed occurrence threshold")
	}

	items := filterItems(input.ItemStats, input.MinOccurrence)
	if input.TopItems > 0 && len(items) > input.TopItems {
		items = items[:input.TopItems]
	}
	out.Items = items
	if len(items) == 0 {
		out.Warnings = append(out.Warnings, "no item passed occurrence threshold")
	}

	return out
}

func (e *Engine) RecommendRunePage(input domain.RunePageRecommendationInput) domain.RunePageRecommendation {
	var out domain.RunePageRecommendation

	primaryStyleID, ok := runes.StyleForKeystone(input.Keystone.Rune)
	if !ok {
		out.Warnings = append(out.Warnings, "no primary rune tree was available for keystone recommendation")
		return out
	}

	secondaryStyleID, ok := runes.RecommendedSecondaryStyle(input.SecondaryTreePlaycount, primaryStyleID)
	if !ok {
		out.Warnings = append(out.Warnings, "no secondary rune tree recommendation was available")
		return out
	}

	selectedPerkIDs := []int{input.Keystone.Rune}
	for _, slot := range []struct {
		stats   []domain.RuneStat
		warning string
	}{
		{stats: input.PrimaryRunes.RowOnes, warning: "no primary row 1 rune recommendation was available"},
		{stats: input.PrimaryRunes.RowTwos, warning: "no primary row 2 rune recommendation was available"},
		{stats: input.PrimaryRunes.RowThrees, warning: "no primary row 3 rune recommendation was available"},
	} {
		stat, ok := selectTopRune(slot.stats, input.MinOccurrence)
		if !ok {
			out.Warnings = append(out.Warnings, slot.warning)
			return out
		}
		selectedPerkIDs = append(selectedPerkIDs, stat.Rune)
	}

	secondaryRunes := selectSecondaryRunes(input.SecondaryRunes, input.MinOccurrence)
	if len(secondaryRunes) != 2 {
		out.Warnings = append(out.Warnings, "no complete secondary rune recommendation was available")
		return out
	}
	for _, stat := range secondaryRunes {
		selectedPerkIDs = append(selectedPerkIDs, stat.Rune)
	}

	for _, slot := range []struct {
		stats   []domain.RuneStat
		warning string
	}{
		{stats: input.Shards.Offense, warning: "no offense shard recommendation was available"},
		{stats: input.Shards.Flex, warning: "no flex shard recommendation was available"},
		{stats: input.Shards.Defense, warning: "no defense shard recommendation was available"},
	} {
		stat, ok := selectTopRune(slot.stats, input.MinOccurrence)
		if !ok {
			out.Warnings = append(out.Warnings, slot.warning)
			return out
		}
		selectedPerkIDs = append(selectedPerkIDs, stat.Rune)
	}

	out.Page = &domain.RunePage{
		PrimaryStyleID:  primaryStyleID,
		SubStyleID:      secondaryStyleID,
		SelectedPerkIDs: selectedPerkIDs,
	}

	return out
}

func filterKeystones(in []domain.KeystoneStat, minOccurrence int) []domain.KeystoneStat {
	out := make([]domain.KeystoneStat, 0, len(in))
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

	return out
}

func selectTopRune(in []domain.RuneStat, minOccurrence int) (domain.RuneStat, bool) {
	out := filterRunesWithFallback(in, minOccurrence)
	if len(out) == 0 {
		return domain.RuneStat{}, false
	}

	sortRuneStats(out)
	return out[0], true
}

func selectSecondaryRunes(in domain.RuneStatsByRow, minOccurrence int) []domain.RuneStat {
	var (
		rows         = [][]domain.RuneStat{in.RowOnes, in.RowTwos, in.RowThrees}
		candidates   = make([]domain.RuneStat, 0, 3)
		selectedRows = make([]bool, len(rows))
	)
	for idx, row := range rows {
		if stat, ok := selectTopRuneStrict(row, minOccurrence); ok {
			candidates = append(candidates, stat)
			selectedRows[idx] = true
		}
	}

	sortRuneStats(candidates)
	if len(candidates) >= 2 {
		return candidates[:2]
	}

	fallbacks := make([]domain.RuneStat, 0, len(rows))
	for idx, row := range rows {
		if selectedRows[idx] {
			continue
		}
		if stat, ok := selectTopRune(row, minOccurrence); ok {
			fallbacks = append(fallbacks, stat)
		}
	}

	sortRuneStats(fallbacks)
	for _, stat := range fallbacks {
		candidates = append(candidates, stat)
		if len(candidates) == 2 {
			break
		}
	}

	sortRuneStats(candidates)
	return candidates
}

func selectTopRuneStrict(in []domain.RuneStat, minOccurrence int) (domain.RuneStat, bool) {
	out := make([]domain.RuneStat, 0, len(in))
	for _, stat := range in {
		if stat.Occurrence >= minOccurrence {
			out = append(out, stat)
		}
	}
	if len(out) == 0 {
		return domain.RuneStat{}, false
	}

	sortRuneStats(out)
	return out[0], true
}

func filterRunesWithFallback(in []domain.RuneStat, minOccurrence int) []domain.RuneStat {
	filtered := make([]domain.RuneStat, 0, len(in))
	for _, stat := range in {
		if stat.Occurrence >= minOccurrence {
			filtered = append(filtered, stat)
		}
	}

	if len(filtered) > 0 || len(in) == 0 {
		return filtered
	}

	return slices.Clone(in)
}

func sortRuneStats(in []domain.RuneStat) {
	sort.Slice(in, func(i, j int) bool {
		if in[i].WPAOverall == in[j].WPAOverall {
			return in[i].Occurrence > in[j].Occurrence
		}
		return in[i].WPAOverall > in[j].WPAOverall
	})
}

func filterSpells(in []domain.SummonerSpellStat, minOccurrence int) []domain.SummonerSpellStat {
	out := make([]domain.SummonerSpellStat, 0, len(in))
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

	return out
}

func filterItems(in []domain.ItemStat, minOccurrence int) []domain.ItemStat {
	out := make([]domain.ItemStat, 0, len(in))
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

	return out
}
