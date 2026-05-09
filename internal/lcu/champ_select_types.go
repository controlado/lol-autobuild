package lcu

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type champSelectMySelectionPatch struct {
	Spell1ID int `json:"spell1Id"`
	Spell2ID int `json:"spell2Id"`
}

type champSelectSession struct {
	LocalPlayerCellID int                          `json:"localPlayerCellId"`
	QueueID           int                          `json:"queueId"`
	MyTeam            []champSelectPlayerSelection `json:"myTeam"`
	TheirTeam         []champSelectPlayerSelection `json:"theirTeam"`
	GameID            json.RawMessage              `json:"gameId"`
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

func champSelectStateFromSession(session champSelectSession) domain.ChampSelectState {
	return domain.ChampSelectState{
		SessionKey:     champSelectSessionKey(session),
		EnemyChampions: enemyChampionsFromSession(session),
	}
}

func enemyChampionsFromSession(session champSelectSession) []domain.ChampionRef {
	enemies := make([]domain.ChampionRef, 0, len(session.TheirTeam))
	for _, member := range session.TheirTeam {
		if member.ChampionID <= 0 {
			continue
		}
		enemies = append(enemies, domain.ChampionRef{ID: member.ChampionID})
	}
	return enemies
}

func champSelectSessionKey(session champSelectSession) string {
	if gameID := gameIDFromRaw(session.GameID); gameID != "" {
		return "game:" + gameID
	}

	var builder strings.Builder
	builder.WriteString("queue:")
	builder.WriteString(strconv.Itoa(session.QueueID))
	builder.WriteString(":local:")
	builder.WriteString(strconv.Itoa(session.LocalPlayerCellID))
	builder.WriteString(":my:")
	writeSortedCellIDs(&builder, session.MyTeam)
	builder.WriteString(":their:")
	writeSortedCellIDs(&builder, session.TheirTeam)
	return builder.String()
}

func writeSortedCellIDs(builder *strings.Builder, team []champSelectPlayerSelection) {
	cellIDs := make([]int, 0, len(team))
	for _, member := range team {
		cellIDs = append(cellIDs, member.CellID)
	}
	slices.Sort(cellIDs)
	for _, cellID := range cellIDs {
		builder.WriteString(strconv.Itoa(cellID))
		builder.WriteByte(',')
	}
}
