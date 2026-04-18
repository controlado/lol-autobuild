package lcu

import (
	"errors"
)

var (
	ErrNotConfigured                     = errors.New("lcu client is not configured")
	ErrLockfileNotFound                  = errors.New("lcu lockfile not found")
	ErrInvalidLockfile                   = errors.New("invalid lcu lockfile")
	ErrChampSelectUnavailable            = errors.New("champ select session is unavailable")
	ErrChampionNotSelected               = errors.New("local champion is not selected")
	ErrPositionDetectionUnsupportedQueue = errors.New("position detection is unsupported for this queue")
	ErrInvalidItemSetRequest             = errors.New("invalid item set apply request")
	ErrItemSetApplyFailed                = errors.New("apply item set failed")
	ErrInvalidSummonerSpellsRequest      = errors.New("invalid summoner spells apply request")
	ErrChampionSelectionChanged          = errors.New("champion selection changed during apply")
	ErrSummonerSpellsApplyFailed         = errors.New("apply summoner spells failed")
)
