package domain

import (
	"slices"
	"testing"
)

func TestMatchupChampionIDsForRosterCapsBeforeRosterOrder(t *testing.T) {
	t.Parallel()

	requested := []int{60, 20, 0, 20, 10, 30, 40, 50}
	roster := []ChampionRef{{ID: 10}, {ID: 20}, {ID: 30}, {ID: 40}, {ID: 50}, {ID: 60}}

	got := MatchupChampionIDsForRoster(requested, roster, MaxMatchupChampionIDs)
	want := []int{10, 20, 30, 40, 60}
	if !slices.Equal(got, want) {
		t.Fatalf("MatchupChampionIDsForRoster() = %+v, want %+v", got, want)
	}
}

func TestMatchupChampionIDsForRosterFiltersInvalidAndDuplicateIDs(t *testing.T) {
	t.Parallel()

	requested := []int{0, -1, 20, 20, 10}
	roster := []ChampionRef{{ID: 20}, {ID: 0}, {ID: 20}, {ID: 10}}

	got := MatchupChampionIDsForRoster(requested, roster, MaxMatchupChampionIDs)
	want := []int{20, 10}
	if !slices.Equal(got, want) {
		t.Fatalf("MatchupChampionIDsForRoster() = %+v, want %+v", got, want)
	}
}

func TestMatchupChampionIDsForRosterReturnsNilWithoutSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested []int
		roster    []ChampionRef
		limit     int
	}{
		{name: "empty requested", requested: nil, roster: []ChampionRef{{ID: 10}}, limit: MaxMatchupChampionIDs},
		{name: "empty roster", requested: []int{10}, roster: nil, limit: MaxMatchupChampionIDs},
		{name: "limit zero", requested: []int{10}, roster: []ChampionRef{{ID: 10}}, limit: 0},
		{name: "no intersection", requested: []int{10}, roster: []ChampionRef{{ID: 20}}, limit: MaxMatchupChampionIDs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := MatchupChampionIDsForRoster(tt.requested, tt.roster, tt.limit); got != nil {
				t.Fatalf("MatchupChampionIDsForRoster() = %+v, want nil", got)
			}
		})
	}
}
