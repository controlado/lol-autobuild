package app

const (
	MessageCodeLCUOff                    = "lcu.off"
	MessageCodeLCULockfileNotFound       = "lcu.lockfile_not_found"
	MessageCodeLCUChampSelectUnavailable = "lcu.champ_select_unavailable"
	MessageCodeLCUChampionNotSelected    = "lcu.champion_not_selected"
	MessageCodeCoachlessLoginMissing     = "coachless.login_missing"
	MessageCodeCoachlessAuthUnavailable  = "coachless.auth_unavailable"
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
	if err == nil {
		return UserMessage{}
	}

	return UserMessage{Text: err.Error()}
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

func coachlessAuthUnavailableMessage() UserMessage {
	return UserMessage{Code: MessageCodeCoachlessAuthUnavailable, Text: "Coachless authentication is unavailable."}
}
