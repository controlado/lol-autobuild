package lcu

import "fmt"

type champSelectMySelectionPatch struct {
	Spell1ID int `json:"spell1Id"`
	Spell2ID int `json:"spell2Id"`
}

type champSelectSession struct {
	LocalPlayerCellID int                          `json:"localPlayerCellId"`
	QueueID           int                          `json:"queueId"`
	MyTeam            []champSelectPlayerSelection `json:"myTeam"`
}

type champSelectPlayerSelection struct {
	CellID           int    `json:"cellId"`
	ChampionID       int    `json:"championId"`
	AssignedPosition string `json:"assignedPosition"`
	IsAutofilled     bool   `json:"isAutofilled"`
	Spell1ID         int    `json:"spell1Id"`
	Spell2ID         int    `json:"spell2Id"`
}

func localPlayerFromSession(session champSelectSession) (champSelectPlayerSelection, error) {
	for _, member := range session.MyTeam {
		if member.CellID == session.LocalPlayerCellID {
			return member, nil
		}
	}

	return champSelectPlayerSelection{}, fmt.Errorf("%w: local player cell %d not found in myTeam", ErrChampSelectUnavailable, session.LocalPlayerCellID)
}
