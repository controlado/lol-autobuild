package coachless

const (
	AuthLoginPath   = "/api/Auth/login"
	authRefreshPath = "/api/Auth/refresh"

	championWinprobPatchesPath            = "/api/ChampionWinprob/GetPatches"
	championWinprobSummonerSpellStatsPath = "/api/ChampionWinprob/GetGlobalSummonerSpellStatistics"
	championWinprobItemStatsPath          = "/api/ChampionWinprob/GetGlobalItemStatistics"

	runeKeystoneDataPath             = "/api/Rune/GetKeystoneData"
	runeSecondaryTreePlaycountPath   = "/api/Rune/GetSecondaryTreePlaycount"
	runeStatsForKeystoneAndTreePath  = "/api/Rune/GetRunesForKeystoneAndTree"
	runeShardsForKeystoneAndTreePath = "/api/Rune/GetShardsForKeystoneAndTree"
)
