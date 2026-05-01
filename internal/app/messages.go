package app

import (
	"errors"
	"strings"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/lcu"
)

const (
	MessageCodeLCUOff                    = "lcu.off"
	MessageCodeLCULockfileNotFound       = "lcu.lockfile_not_found"
	MessageCodeLCUChampSelectUnavailable = "lcu.champ_select_unavailable"
	MessageCodeLCUChampionNotSelected    = "lcu.champion_not_selected"
	MessageCodeCoachlessLoginMissing     = "coachless.login_missing"
	MessageCodeSyncAlreadyRunning        = "sync.already_running"
	MessageCodeWatcherPreStartFailed     = "watch.pre_start_failed"
	MessageCodeWatcherStartFailed        = "watch.start_failed"
)

type UserMessage struct {
	Code string
	Text string
}

func (m UserMessage) Empty() bool {
	return m.Code == "" && m.Text == ""
}

func userMessageFromErr(err error) UserMessage {
	switch {
	case err == nil:
		return UserMessage{}
	case errors.Is(err, lcu.ErrNotConfigured):
		return UserMessage{Code: MessageCodeLCUOff, Text: "LCU is off."}
	case errors.Is(err, lcu.ErrLockfileNotFound):
		return UserMessage{Code: MessageCodeLCULockfileNotFound, Text: "League Client is not open."}
	case errors.Is(err, lcu.ErrChampSelectUnavailable):
		return UserMessage{Code: MessageCodeLCUChampSelectUnavailable, Text: "Champ select is not ready."}
	case errors.Is(err, lcu.ErrChampionNotSelected):
		return UserMessage{Code: MessageCodeLCUChampionNotSelected, Text: "Select a champion first."}
	case errors.Is(err, auth.ErrNotImplemented), strings.Contains(err.Error(), "unable to acquire valid access token"):
		return UserMessage{Code: MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."}
	default:
		return UserMessage{Text: err.Error()}
	}
}

func syncAlreadyRunningMessage() UserMessage {
	return UserMessage{Code: MessageCodeSyncAlreadyRunning, Text: "Another sync is already running"}
}

func watcherPreStartFailedMessage() UserMessage {
	return UserMessage{Code: MessageCodeWatcherPreStartFailed, Text: "Watcher pre-start failed."}
}

func watcherStartFailedMessage() UserMessage {
	return UserMessage{Code: MessageCodeWatcherStartFailed, Text: "Watcher start failed."}
}

func coachlessLoginMissingMessage() UserMessage {
	return UserMessage{Code: MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."}
}
