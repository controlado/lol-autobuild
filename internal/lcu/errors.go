package lcu

import "errors"

var (
	ErrNotConfigured                 = errors.New("lcu client is not configured")
	ErrLockfileNotFound              = errors.New("lcu lockfile not found")
	ErrInvalidLockfile               = errors.New("invalid lcu lockfile")
	ErrChampSelectUnavailable        = errors.New("champ select session is unavailable")
	ErrChampionNotSelected           = errors.New("local champion is not selected")
	ErrRoleDetectionUnsupportedQueue = errors.New("role detection is unsupported for this queue")
	ErrRoleNotAssigned               = errors.New("local role is not assigned")
	ErrRoleUnknown                   = errors.New("local role is unknown")
	ErrInvalidSummonerSpellsRequest  = errors.New("invalid summoner spells apply request")
	ErrChampionSelectionChanged      = errors.New("champion selection changed during apply")
	ErrSummonerSpellsApplyFailed     = errors.New("apply summoner spells failed")
)
