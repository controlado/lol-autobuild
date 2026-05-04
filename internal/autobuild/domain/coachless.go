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

const (
	RuneStylePrecision   = 8000
	RuneStyleDomination  = 8100
	RuneStyleSorcery     = 8200
	RuneStyleInspiration = 8300
	RuneStyleResolve     = 8400
)

const (
	RuneKeystonePressTheAttack    = 8005
	RuneKeystoneLethalTempo       = 8008
	RuneKeystoneFleetFootwork     = 8021
	RuneKeystoneConqueror         = 8010
	RuneKeystoneElectrocute       = 8112
	RuneKeystoneDarkHarvest       = 8128
	RuneKeystoneHailOfBlades      = 9923
	RuneKeystoneSummonAery        = 8214
	RuneKeystoneArcaneComet       = 8229
	RuneKeystoneStormraidersSurge = 8230
	RuneKeystoneDeathfireTouch    = 8992
	RuneKeystoneGraspOfTheUndying = 8437
	RuneKeystoneAftershock        = 8439
	RuneKeystoneGuardian          = 8465
	RuneKeystoneGlacialAugment    = 8351
	RuneKeystoneUnsealedSpellbook = 8360
	RuneKeystoneFirstStrike       = 8369
)

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
