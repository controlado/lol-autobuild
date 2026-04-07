package recommend

import (
	"testing"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func TestRecommendSelectsTopByWPA(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.Recommend(ports.RecommendationInput{
		MinOccurrence: 100,
		TopItems:      2,
		TopSpells:     2,
		KeystoneStats: []ports.KeystoneStat{
			{Rune: 8005, WPAOverall: 0.2, Occurrence: 500},
			{Rune: 8437, WPAOverall: 1.1, Occurrence: 300},
		},
		SpellStats: []ports.SummonerSpellStat{
			{SummonerSpell: 4, WPAOverall: 0.3, Occurrence: 400},
			{SummonerSpell: 14, WPAOverall: 0.8, Occurrence: 200},
			{SummonerSpell: 6, WPAOverall: 0.7, Occurrence: 50},
		},
		ItemStats: []ports.ItemStat{
			{ItemID: 1055, WPAOverall: 0.2, Occurrence: 1000},
			{ItemID: 1036, WPAOverall: 0.6, Occurrence: 200},
			{ItemID: 1054, WPAOverall: 0.5, Occurrence: 300},
		},
	})

	if got.Keystone == nil || got.Keystone.Rune != 8437 {
		t.Fatalf("unexpected keystone: %#v", got.Keystone)
	}

	if len(got.SummonerSpells) != 2 || got.SummonerSpells[0].SummonerSpell != 14 {
		t.Fatalf("unexpected spells: %#v", got.SummonerSpells)
	}

	if len(got.Items) != 2 || got.Items[0].ItemID != 1036 || got.Items[1].ItemID != 1054 {
		t.Fatalf("unexpected items: %#v", got.Items)
	}
}

func TestRecommendAddsWarningsWhenNothingPasses(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.Recommend(ports.RecommendationInput{MinOccurrence: 9999, TopItems: 3, TopSpells: 2})

	if len(got.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(got.Warnings))
	}
}
