package domain

type (
	ChampionRef struct {
		ID   int
		Name string
	}
	ChampSelectState struct {
		SessionKey     string
		EnemyChampions []ChampionRef
	}
)

const MaxMatchupChampionIDs = 5

func MatchupChampionIDsForRoster(requested []int, roster []ChampionRef, limit int) []int {
	if limit <= 0 || len(requested) == 0 || len(roster) == 0 {
		return nil
	}

	requestedSet := make(map[int]struct{}, min(len(requested), limit))
	for _, id := range requested {
		if id <= 0 {
			continue
		}
		if _, ok := requestedSet[id]; ok {
			continue
		}
		requestedSet[id] = struct{}{}
		if len(requestedSet) == limit {
			break
		}
	}
	if len(requestedSet) == 0 {
		return nil
	}

	out := make([]int, 0, len(requestedSet))
	seenRoster := make(map[int]struct{}, len(roster))
	for _, champion := range roster {
		if champion.ID <= 0 {
			continue
		}
		if _, seen := seenRoster[champion.ID]; seen {
			continue
		}
		seenRoster[champion.ID] = struct{}{}
		if _, selected := requestedSet[champion.ID]; selected {
			out = append(out, champion.ID)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
