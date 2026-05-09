package recommend

import (
	"reflect"
	"slices"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestRecommendSelectsTopByWPA(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.Recommend(domain.RecommendationInput{
		MinOccurrence: 100,
		TopItems:      2,
		TopSpells:     2,
		KeystoneStats: []domain.KeystoneStat{
			{Rune: 8005, WPAOverall: 0.2, Occurrence: 500},
			{Rune: 8437, WPAOverall: 1.1, Occurrence: 300},
		},
		SpellStats: []domain.SummonerSpellStat{
			{SummonerSpell: 4, WPAOverall: 0.3, Occurrence: 400},
			{SummonerSpell: 14, WPAOverall: 0.8, Occurrence: 200},
			{SummonerSpell: 6, WPAOverall: 0.7, Occurrence: 50},
		},
		ItemStats: []domain.ItemStat{
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
	got := eng.Recommend(domain.RecommendationInput{MinOccurrence: 9999, TopItems: 3, TopSpells: 2})

	if len(got.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(got.Warnings))
	}
}

func TestRecommendRunePageSelectsCompletePage(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(domain.RunePageRecommendationInput{
		Keystone: domain.KeystoneStat{Rune: 8005, WPAOverall: 1.2, Occurrence: 1000},
		SecondaryTreePlaycount: []domain.RuneTreePlaycount{
			{Tree: domain.RuneStylePrecision, Occurrence: 5000},
			{Tree: domain.RuneStyleDomination, Occurrence: 300},
			{Tree: domain.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: domain.RuneStatsByRow{
			RowOnes: []domain.RuneStat{
				{Rune: 9101, WPAOverall: 0.2, Occurrence: 500},
				{Rune: 9111, WPAOverall: 0.5, Occurrence: 500},
			},
			RowTwos: []domain.RuneStat{
				{Rune: 9104, WPAOverall: 0.7, Occurrence: 500},
				{Rune: 9103, WPAOverall: 0.7, Occurrence: 300},
			},
			RowThrees: []domain.RuneStat{
				{Rune: 8299, WPAOverall: 0.4, Occurrence: 500},
			},
		},
		SecondaryRunes: domain.RuneStatsByRow{
			RowOnes: []domain.RuneStat{
				{Rune: 8224, WPAOverall: 0.2, Occurrence: 500},
			},
			RowTwos: []domain.RuneStat{
				{Rune: 8233, WPAOverall: 0.8, Occurrence: 500},
			},
			RowThrees: []domain.RuneStat{
				{Rune: 8237, WPAOverall: 0.6, Occurrence: 500},
			},
		},
		Shards: domain.ShardStats{
			Offense: []domain.RuneStat{{Rune: 5005, WPAOverall: 0.2, Occurrence: 500}},
			Flex:    []domain.RuneStat{{Rune: 5008, WPAOverall: 0.4, Occurrence: 500}},
			Defense: []domain.RuneStat{{Rune: 5002, WPAOverall: 0.1, Occurrence: 500}},
		},
		MinOccurrence: 100,
	})

	if len(got.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", got.Warnings)
	}
	if got.Page == nil {
		t.Fatal("expected rune page recommendation")
	}

	want := domain.RunePage{
		PrimaryStyleID:  domain.RuneStylePrecision,
		SubStyleID:      domain.RuneStyleSorcery,
		SelectedPerkIDs: []int{8005, 9111, 9104, 8299, 8233, 8237, 5005, 5008, 5002},
	}
	if !reflect.DeepEqual(*got.Page, want) {
		t.Fatalf("page = %#v, want %#v", *got.Page, want)
	}
}

func TestRecommendRunePagePrefersTwoQualifiedSecondaryRows(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(validRunePageRecommendationInput(domain.RuneStatsByRow{
		RowOnes: []domain.RuneStat{
			{Rune: 8224, WPAOverall: 0.4, Occurrence: 1200},
		},
		RowTwos: []domain.RuneStat{
			{Rune: 8233, WPAOverall: 0.3, Occurrence: 1100},
		},
		RowThrees: []domain.RuneStat{
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
	if !slices.Equal(got.Page.SelectedPerkIDs, want) {
		t.Fatalf("selectedPerkIDs = %#v, want %#v", got.Page.SelectedPerkIDs, want)
	}
}

func TestRecommendRunePageUsesSecondaryFallbackToCompletePage(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(validRunePageRecommendationInput(domain.RuneStatsByRow{
		RowOnes: []domain.RuneStat{
			{Rune: 8224, WPAOverall: 0.4, Occurrence: 1200},
		},
		RowTwos: []domain.RuneStat{
			{Rune: 8233, WPAOverall: 2.0, Occurrence: 20},
		},
		RowThrees: []domain.RuneStat{
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
	if !slices.Equal(got.Page.SelectedPerkIDs, want) {
		t.Fatalf("selectedPerkIDs = %#v, want %#v", got.Page.SelectedPerkIDs, want)
	}
}

func TestRecommendRunePageAddsWarningWhenSlotMissing(t *testing.T) {
	t.Parallel()

	eng := NewEngine()
	got := eng.RecommendRunePage(domain.RunePageRecommendationInput{
		Keystone: domain.KeystoneStat{Rune: 8437, WPAOverall: 1.2, Occurrence: 1000},
		SecondaryTreePlaycount: []domain.RuneTreePlaycount{
			{Tree: domain.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: domain.RuneStatsByRow{
			RowOnes: []domain.RuneStat{{Rune: 8446, WPAOverall: 0.9, Occurrence: 500}},
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

func validRunePageRecommendationInput(secondary domain.RuneStatsByRow, minOccurrence int) domain.RunePageRecommendationInput {
	return domain.RunePageRecommendationInput{
		Keystone: domain.KeystoneStat{Rune: 8005, WPAOverall: 1.2, Occurrence: 2000},
		SecondaryTreePlaycount: []domain.RuneTreePlaycount{
			{Tree: domain.RuneStyleSorcery, Occurrence: 900},
		},
		PrimaryRunes: domain.RuneStatsByRow{
			RowOnes:   []domain.RuneStat{{Rune: 9101, WPAOverall: 0.7, Occurrence: 2000}},
			RowTwos:   []domain.RuneStat{{Rune: 9104, WPAOverall: 0.6, Occurrence: 2000}},
			RowThrees: []domain.RuneStat{{Rune: 8299, WPAOverall: 0.5, Occurrence: 2000}},
		},
		SecondaryRunes: secondary,
		Shards: domain.ShardStats{
			Offense: []domain.RuneStat{{Rune: 5005, WPAOverall: 0.7, Occurrence: 2000}},
			Flex:    []domain.RuneStat{{Rune: 5008, WPAOverall: 0.6, Occurrence: 2000}},
			Defense: []domain.RuneStat{{Rune: 5002, WPAOverall: 0.5, Occurrence: 2000}},
		},
		MinOccurrence: minOccurrence,
	}
}
