package domain

type PatchInfo struct {
	Label string
	Major int
	Patch int
}

type PatchFilter struct {
	Major          int
	Patch          int
	PatchAdditions int
}

type CommonFilters struct {
	Patch              PatchFilter
	ChampionIDs        []int
	MatchupChampionIDs []int
	LeagueTiers        []int
	Regions            []string
	Role               int
}

type KeystoneStat struct {
	Rune       int
	WPAOverall float64
	Occurrence int
}

type RuneStat struct {
	Rune       int
	WPAOverall float64
	Occurrence int
}

type RuneTreePlaycount struct {
	Tree       int
	Occurrence float64
}

type RuneStatsByRow struct {
	RowOnes   []RuneStat
	RowTwos   []RuneStat
	RowThrees []RuneStat
}

type ShardStats struct {
	Offense []RuneStat
	Flex    []RuneStat
	Defense []RuneStat
}

type SummonerSpellStat struct {
	SummonerSpell int
	WPAOverall    float64
	Occurrence    int
}

type ItemStat struct {
	ItemID     int
	WPAOverall float64
	Occurrence int
}

type KeystoneRequest struct {
	CommonFilters CommonFilters
}

type SecondaryTreePlaycountRequest struct {
	CommonFilters CommonFilters
	Tree          int
	Keystone      int
}

type RuneStatsRequest struct {
	CommonFilters CommonFilters
	Keystone      int
	MainTree      int
	TreeToLoad    int
}

type ShardStatsRequest struct {
	CommonFilters CommonFilters
	Keystone      int
}

type SummonerSpellStatsRequest struct {
	CommonFilters CommonFilters
	PairedSpell   *int
}

type ItemStatsRequest struct {
	CommonFilters         CommonFilters
	ItemSlots             []int
	ItemType              int
	Keystone              *int
	StarterID             *int
	FirstPurchaseID       *int
	FirstLegendaryID      *int
	SecondLegendaryID     *int
	LoadFirstEpicPurchase bool
	IncludeSupportItems   bool
}
