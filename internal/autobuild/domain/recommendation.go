package domain

type RecommendationInput struct {
	KeystoneStats []KeystoneStat
	SpellStats    []SummonerSpellStat
	ItemStats     []ItemStat
	MinOccurrence int
	TopItems      int
	TopSpells     int
}

type Recommendation struct {
	Keystone       *KeystoneStat
	SummonerSpells []SummonerSpellStat
	Items          []ItemStat
	Warnings       []string
}

type RunePageRecommendationInput struct {
	Keystone               KeystoneStat
	SecondaryTreePlaycount []RuneTreePlaycount
	PrimaryRunes           RuneStatsByRow
	SecondaryRunes         RuneStatsByRow
	Shards                 ShardStats
	MinOccurrence          int
}

type RunePageRecommendation struct {
	Page     *RunePage
	Warnings []string
}
