package coachless

import (
	"slices"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type apiRefreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type apiPatchInfo struct {
	Label      string `json:"label"`
	Major      int    `json:"major"`
	Patch      int    `json:"patch"`
	MatchCount int    `json:"matchCount"`
}

type apiPatchFilter struct {
	Major          int `json:"major"`
	Patch          int `json:"patch"`
	PatchAdditions int `json:"patchAdditions"`
}

type apiCommonFilters struct {
	Patch              apiPatchFilter `json:"patch"`
	ChampionIDs        []int          `json:"championIds"`
	MatchupChampionIDs []int          `json:"matchupChampionIds,omitempty"`
	LeagueTiers        []int          `json:"leagueTiers"`
	Regions            []string       `json:"regions,omitempty"`
	Role               int            `json:"role"`
}

type apiKeystoneStat struct {
	Rune       int     `json:"rune"`
	WPAOverall float64 `json:"wpaOverall"`
	Occurrence int     `json:"occurrence"`
}

type apiRuneStat struct {
	Rune       int     `json:"rune"`
	WPAOverall float64 `json:"wpaOverall"`
	Occurrence int     `json:"occurrence"`
}

type apiRuneTreePlaycount struct {
	Tree       int     `json:"tree"`
	Occurrence float64 `json:"occurrence"`
}

type apiRuneStatsByRow struct {
	RowOnes   []apiRuneStat `json:"rowOnes"`
	RowTwos   []apiRuneStat `json:"rowTwos"`
	RowThrees []apiRuneStat `json:"rowThrees"`
}

type apiShardStats struct {
	Offense []apiRuneStat `json:"offense"`
	Flex    []apiRuneStat `json:"flex"`
	Defense []apiRuneStat `json:"defense"`
}

type apiSummonerSpellStat struct {
	SummonerSpell int     `json:"summonerSpell"`
	WPAOverall    float64 `json:"wpaOverall"`
	Occurrence    int     `json:"occurrence"`
}

type apiItemStat struct {
	ItemID     int     `json:"itemId"`
	WPAOverall float64 `json:"wpaOverall"`
	Occurrence int     `json:"occurrence"`
}

type apiKeystoneRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
}

type apiSecondaryTreePlaycountRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
	Tree          int              `json:"tree"`
	Keystone      int              `json:"keystone"`
}

type apiRuneStatsRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
	Keystone      int              `json:"keystone"`
	MainTree      int              `json:"mainTree"`
	TreeToLoad    int              `json:"treeToLoad"`
}

type apiShardStatsRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
	Keystone      int              `json:"keystone"`
}

type apiSummonerSpellStatsRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
	PairedSpell   *int             `json:"pairedSpell"`
}

type apiItemStatsRequest struct {
	CommonFilters         apiCommonFilters `json:"commonFilters"`
	ItemSlots             []int            `json:"itemSlots,omitempty"`
	ItemType              int              `json:"itemType"`
	Keystone              *int             `json:"keystone"`
	StarterID             *int             `json:"starterId"`
	FirstPurchaseID       *int             `json:"firstPurchaseId"`
	FirstLegendaryID      *int             `json:"firstLegendaryId"`
	SecondLegendaryID     *int             `json:"secondLegendaryId"`
	LoadFirstEpicPurchase bool             `json:"loadFirstEpicPurchase"`
	IncludeSupportItems   bool             `json:"includeSupportItems"`
}

func apiKeystoneRequestFromDomain(req domain.KeystoneRequest) apiKeystoneRequest {
	return apiKeystoneRequest{CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters)}
}

func apiSecondaryTreePlaycountRequestFromDomain(req domain.SecondaryTreePlaycountRequest) apiSecondaryTreePlaycountRequest {
	return apiSecondaryTreePlaycountRequest{
		CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters),
		Tree:          req.Tree,
		Keystone:      req.Keystone,
	}
}

func apiRuneStatsRequestFromDomain(req domain.RuneStatsRequest) apiRuneStatsRequest {
	return apiRuneStatsRequest{
		CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters),
		Keystone:      req.Keystone,
		MainTree:      req.MainTree,
		TreeToLoad:    req.TreeToLoad,
	}
}

func apiShardStatsRequestFromDomain(req domain.ShardStatsRequest) apiShardStatsRequest {
	return apiShardStatsRequest{
		CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters),
		Keystone:      req.Keystone,
	}
}

func apiSummonerSpellStatsRequestFromDomain(req domain.SummonerSpellStatsRequest) apiSummonerSpellStatsRequest {
	return apiSummonerSpellStatsRequest{
		CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters),
		PairedSpell:   req.PairedSpell,
	}
}

func apiItemStatsRequestFromDomain(req domain.ItemStatsRequest) apiItemStatsRequest {
	return apiItemStatsRequest{
		CommonFilters:         apiCommonFiltersFromDomain(req.CommonFilters),
		ItemSlots:             slices.Clone(req.ItemSlots),
		ItemType:              req.ItemType,
		Keystone:              req.Keystone,
		StarterID:             req.StarterID,
		FirstPurchaseID:       req.FirstPurchaseID,
		FirstLegendaryID:      req.FirstLegendaryID,
		SecondLegendaryID:     req.SecondLegendaryID,
		LoadFirstEpicPurchase: req.LoadFirstEpicPurchase,
		IncludeSupportItems:   req.IncludeSupportItems,
	}
}

func apiCommonFiltersFromDomain(filters domain.CommonFilters) apiCommonFilters {
	return apiCommonFilters{
		Patch:              apiPatchFilterFromDomain(filters.Patch),
		ChampionIDs:        slices.Clone(filters.ChampionIDs),
		MatchupChampionIDs: slices.Clone(filters.MatchupChampionIDs),
		LeagueTiers:        slices.Clone(filters.LeagueTiers),
		Regions:            slices.Clone(filters.Regions),
		Role:               filters.Role,
	}
}

func apiPatchFilterFromDomain(filter domain.PatchFilter) apiPatchFilter {
	return apiPatchFilter{
		Major:          filter.Major,
		Patch:          filter.Patch,
		PatchAdditions: filter.PatchAdditions,
	}
}

func patchInfosFromAPI(in []apiPatchInfo) []domain.PatchInfo {
	out := make([]domain.PatchInfo, 0, len(in))
	for _, patch := range in {
		out = append(out, domain.PatchInfo{
			Label: patch.Label,
			Major: patch.Major,
			Patch: patch.Patch,
		})
	}
	return out
}

func keystoneStatsFromAPI(in []apiKeystoneStat) []domain.KeystoneStat {
	out := make([]domain.KeystoneStat, 0, len(in))
	for _, stat := range in {
		out = append(out, domain.KeystoneStat{
			Rune:       stat.Rune,
			WPAOverall: stat.WPAOverall,
			Occurrence: stat.Occurrence,
		})
	}
	return out
}

func runeTreePlaycountsFromAPI(in []apiRuneTreePlaycount) []domain.RuneTreePlaycount {
	out := make([]domain.RuneTreePlaycount, 0, len(in))
	for _, stat := range in {
		out = append(out, domain.RuneTreePlaycount{
			Tree:       stat.Tree,
			Occurrence: stat.Occurrence,
		})
	}
	return out
}

func runeStatsByRowFromAPI(in apiRuneStatsByRow) domain.RuneStatsByRow {
	return domain.RuneStatsByRow{
		RowOnes:   runeStatsFromAPI(in.RowOnes),
		RowTwos:   runeStatsFromAPI(in.RowTwos),
		RowThrees: runeStatsFromAPI(in.RowThrees),
	}
}

func shardStatsFromAPI(in apiShardStats) domain.ShardStats {
	return domain.ShardStats{
		Offense: runeStatsFromAPI(in.Offense),
		Flex:    runeStatsFromAPI(in.Flex),
		Defense: runeStatsFromAPI(in.Defense),
	}
}

func runeStatsFromAPI(in []apiRuneStat) []domain.RuneStat {
	out := make([]domain.RuneStat, 0, len(in))
	for _, stat := range in {
		out = append(out, domain.RuneStat{
			Rune:       stat.Rune,
			WPAOverall: stat.WPAOverall,
			Occurrence: stat.Occurrence,
		})
	}
	return out
}

func summonerSpellStatsFromAPI(in []apiSummonerSpellStat) []domain.SummonerSpellStat {
	out := make([]domain.SummonerSpellStat, 0, len(in))
	for _, stat := range in {
		out = append(out, domain.SummonerSpellStat{
			SummonerSpell: stat.SummonerSpell,
			WPAOverall:    stat.WPAOverall,
			Occurrence:    stat.Occurrence,
		})
	}
	return out
}

func itemStatsFromAPI(in []apiItemStat) []domain.ItemStat {
	out := make([]domain.ItemStat, 0, len(in))
	for _, stat := range in {
		out = append(out, domain.ItemStat{
			ItemID:     stat.ItemID,
			WPAOverall: stat.WPAOverall,
			Occurrence: stat.Occurrence,
		})
	}
	return out
}
