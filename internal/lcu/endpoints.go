package lcu

const (
	champSelectSessionURI     = "/lol-champ-select/v1/session"
	champSelectMySelectionURI = champSelectSessionURI + "/my-selection"
	championSummaryURI        = "/lol-game-data/assets/v1/champion-summary.json"
	championDetailsURIFormat  = "/lol-game-data/assets/v1/champions/%d.json"
	currentRunePageURI        = "/lol-perks/v1/currentpage"
	currentSummonerURI        = "/lol-summoner/v1/current-summoner"
	itemSetsURIFormat         = "/lol-item-sets/v1/item-sets/%d/sets"
	riotClientUXStateURI      = "/riotclient/ux-state"
	runePagesURI              = "/lol-perks/v1/pages"
	runePageURIFormat         = runePagesURI + "/%d"
)
