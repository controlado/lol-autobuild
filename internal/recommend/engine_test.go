package recommend

import (
	"reflect"
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

func TestRecommendRunePageSelectsCompletePage(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(ports.RunePageRecommendationInput{
		Keystone: ports.KeystoneStat{Rune: 8005, WPAOverall: 1.2, Occurrence: 1000},
		SecondaryTreePlaycount: []ports.RuneTreePlaycount{
			{Tree: ports.RuneStylePrecision, Occurrence: 5000},
			{Tree: ports.RuneStyleDomination, Occurrence: 300},
			{Tree: ports.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: ports.RuneStatsByRow{
			RowOnes: []ports.RuneStat{
				{Rune: 9101, WPAOverall: 0.2, Occurrence: 500},
				{Rune: 9111, WPAOverall: 0.5, Occurrence: 500},
			},
			RowTwos: []ports.RuneStat{
				{Rune: 9104, WPAOverall: 0.7, Occurrence: 500},
				{Rune: 9103, WPAOverall: 0.7, Occurrence: 300},
			},
			RowThrees: []ports.RuneStat{
				{Rune: 8299, WPAOverall: 0.4, Occurrence: 500},
			},
		},
		SecondaryRunes: ports.RuneStatsByRow{
			RowOnes: []ports.RuneStat{
				{Rune: 8224, WPAOverall: 0.2, Occurrence: 500},
			},
			RowTwos: []ports.RuneStat{
				{Rune: 8233, WPAOverall: 0.8, Occurrence: 500},
			},
			RowThrees: []ports.RuneStat{
				{Rune: 8237, WPAOverall: 0.6, Occurrence: 500},
			},
		},
		Shards: ports.ShardStats{
			Offense: []ports.RuneStat{{Rune: 5005, WPAOverall: 0.2, Occurrence: 500}},
			Flex:    []ports.RuneStat{{Rune: 5008, WPAOverall: 0.4, Occurrence: 500}},
			Defense: []ports.RuneStat{{Rune: 5002, WPAOverall: 0.1, Occurrence: 500}},
		},
		MinOccurrence: 100,
	})

	if len(got.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", got.Warnings)
	}
	if got.Page == nil {
		t.Fatal("expected rune page recommendation")
	}

	want := ports.RunePage{
		PrimaryStyleID:  ports.RuneStylePrecision,
		SubStyleID:      ports.RuneStyleSorcery,
		SelectedPerkIDs: []int{8005, 9111, 9104, 8299, 8233, 8237, 5005, 5008, 5002},
	}
	if !reflect.DeepEqual(*got.Page, want) {
		t.Fatalf("page = %#v, want %#v", *got.Page, want)
	}
}

func TestRecommendRunePagePrefersTwoQualifiedSecondaryRows(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(validRunePageRecommendationInput(ports.RuneStatsByRow{
		RowOnes: []ports.RuneStat{
			{Rune: 8224, WPAOverall: 0.4, Occurrence: 1200},
		},
		RowTwos: []ports.RuneStat{
			{Rune: 8233, WPAOverall: 0.3, Occurrence: 1100},
		},
		RowThrees: []ports.RuneStat{
			{Rune: 8237, WPAOverall: 2.0, Occurrence: 20},
		},
	}, 1000))

	if len(got.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", got.Warnings)
	}
	if got.Page == nil {
		t.Fatal("expected rune page recommendation")
	}

	want := []int{8005, 9101, 9104, 8299, 8224, 8233, 5005, 5008, 5002}
	if !reflect.DeepEqual(got.Page.SelectedPerkIDs, want) {
		t.Fatalf("selectedPerkIDs = %#v, want %#v", got.Page.SelectedPerkIDs, want)
	}
}

func TestRecommendRunePageUsesSecondaryFallbackToCompletePage(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(validRunePageRecommendationInput(ports.RuneStatsByRow{
		RowOnes: []ports.RuneStat{
			{Rune: 8224, WPAOverall: 0.4, Occurrence: 1200},
		},
		RowTwos: []ports.RuneStat{
			{Rune: 8233, WPAOverall: 2.0, Occurrence: 20},
		},
		RowThrees: []ports.RuneStat{
			{Rune: 8237, WPAOverall: 0.6, Occurrence: 30},
		},
	}, 1000))

	if len(got.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", got.Warnings)
	}
	if got.Page == nil {
		t.Fatal("expected rune page recommendation")
	}

	want := []int{8005, 9101, 9104, 8299, 8233, 8224, 5005, 5008, 5002}
	if !reflect.DeepEqual(got.Page.SelectedPerkIDs, want) {
		t.Fatalf("selectedPerkIDs = %#v, want %#v", got.Page.SelectedPerkIDs, want)
	}
}

func TestRecommendRunePageAddsWarningWhenSlotMissing(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(ports.RunePageRecommendationInput{
		Keystone: ports.KeystoneStat{Rune: 8437, WPAOverall: 1.2, Occurrence: 1000},
		SecondaryTreePlaycount: []ports.RuneTreePlaycount{
			{Tree: ports.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: ports.RuneStatsByRow{
			RowOnes: []ports.RuneStat{{Rune: 8446, WPAOverall: 0.9, Occurrence: 500}},
		},
		MinOccurrence: 100,
	})

	if got.Page != nil {
		t.Fatalf("expected no page when a required slot is missing, got %#v", got.Page)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "no primary row 2 rune recommendation was available" {
		t.Fatalf("unexpected warnings: %#v", got.Warnings)
	}
}

func validRunePageRecommendationInput(secondary ports.RuneStatsByRow, minOccurrence int) ports.RunePageRecommendationInput {
	return ports.RunePageRecommendationInput{
		Keystone: ports.KeystoneStat{Rune: 8005, WPAOverall: 1.2, Occurrence: 2000},
		SecondaryTreePlaycount: []ports.RuneTreePlaycount{
			{Tree: ports.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: ports.RuneStatsByRow{
			RowOnes:   []ports.RuneStat{{Rune: 9101, WPAOverall: 0.7, Occurrence: 2000}},
			RowTwos:   []ports.RuneStat{{Rune: 9104, WPAOverall: 0.6, Occurrence: 2000}},
			RowThrees: []ports.RuneStat{{Rune: 8299, WPAOverall: 0.5, Occurrence: 2000}},
		},
		SecondaryRunes: secondary,
		Shards: ports.ShardStats{
			Offense: []ports.RuneStat{{Rune: 5005, WPAOverall: 0.7, Occurrence: 2000}},
			Flex:    []ports.RuneStat{{Rune: 5008, WPAOverall: 0.6, Occurrence: 2000}},
			Defense: []ports.RuneStat{{Rune: 5002, WPAOverall: 0.5, Occurrence: 2000}},
		},
		MinOccurrence: minOccurrence,
	}
}
