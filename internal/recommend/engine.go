package recommend

import (
	"sort"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Recommend(input ports.RecommendationInput) ports.Recommendation {
	out := ports.Recommendation{}

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

func filterKeystones(in []ports.KeystoneStat, minOccurrence int) []ports.KeystoneStat {
	out := make([]ports.KeystoneStat, 0, len(in))
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

func filterSpells(in []ports.SummonerSpellStat, minOccurrence int) []ports.SummonerSpellStat {
	out := make([]ports.SummonerSpellStat, 0, len(in))
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

func filterItems(in []ports.ItemStat, minOccurrence int) []ports.ItemStat {
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

	return out
}
