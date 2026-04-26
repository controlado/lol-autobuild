package app

import (
	"errors"
	"strings"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/lcu"
)

func messageFromErr(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, lcu.ErrNotConfigured):
		return "LCU is off."
	case errors.Is(err, lcu.ErrLockfileNotFound):
		return "League Client is not open."
	case errors.Is(err, lcu.ErrChampSelectUnavailable):
		return "Champ select is not ready."
	case errors.Is(err, lcu.ErrChampionNotSelected):
		return "Select a champion first."
	case errors.Is(err, auth.ErrNotImplemented):
		return "Coachless login is missing."
	case strings.Contains(err.Error(), "unable to acquire valid access token"):
		return "Coachless login is missing."
	default:
		return err.Error()
	}
}
