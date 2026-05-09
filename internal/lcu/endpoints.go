package lcu

const (
	champSelectSessionPath     = "/lol-champ-select/v1/session"
	champSelectMySelectionPath = champSelectSessionPath + "/my-selection"

	championSummaryPath       = "/lol-game-data/assets/v1/champion-summary.json"
	championDetailsPathFormat = "/lol-game-data/assets/v1/champions/%d.json"

	currentRunePagePath = "/lol-perks/v1/currentpage"
	runePagesPath       = "/lol-perks/v1/pages"
	runePagePathFormat  = runePagesPath + "/%d"

	currentSummonerPath = "/lol-summoner/v1/current-summoner"

	itemSetsPathFormat = "/lol-item-sets/v1/item-sets/%d/sets"

	riotClientUXStatePath = "/riotclient/ux-state"
)
