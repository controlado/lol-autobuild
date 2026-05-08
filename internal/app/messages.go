package app

import "fmt"

const (
	MessageCodeLCUOff                          = "lcu.off"
	MessageCodeLCUNotConnected                 = "lcu.not_connected"
	MessageCodeLCULockfileNotFound             = "lcu.lockfile_not_found"
	MessageCodeLCUNotReachable                 = "lcu.not_reachable"
	MessageCodeLCUChampSelectUnavailable       = "lcu.champ_select_unavailable"
	MessageCodeLCUChampionNotSelected          = "lcu.champion_not_selected"
	MessageCodeCoachlessLoginMissing           = "coachless.login_missing"
	MessageCodeCoachlessAuthUnavailable        = "coachless.auth_unavailable"
	MessageCodeSyncAlreadyRunning              = "sync.already_running"
	MessageCodeSyncRunePageLimitReached        = "sync.rune_page_limit_reached"
	MessageCodeWatcherPreStartFailed           = "watch.pre_start_failed"
	MessageCodeWatcherStartFailed              = "watch.start_failed"
	MessageCodeWatchNoticeConnected            = "watch.notice.connected"
	MessageCodeWatchNoticeReconnecting         = "watch.notice.reconnecting"
	MessageCodeWatchNoticeSnapshotFinalization = "watch.notice.snapshot_finalization"
	MessageCodeWatchNoticeSnapshotWaiting      = "watch.notice.snapshot_waiting"
	MessageCodeUpdateUpToDate                  = "update.up_to_date"
	MessageCodeUpdateCannotCheck               = "update.cannot_check"
)

type UserMessage struct {
	Code string
	Text string
}

func NewMessageDescriptor(key, fallback string) *MessageDescriptor {
	if key == "" && fallback == "" {
		return nil
	}

	return &MessageDescriptor{Key: key, Fallback: fallback}
}

func (m UserMessage) Empty() bool {
	return m.Code == "" && m.Text == ""
}

func (m UserMessage) Descriptor() *MessageDescriptor {
	return NewMessageDescriptor(m.Code, m.Text)
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

func updateAvailableMessage(latestVersion string) *MessageDescriptor {
	if latestVersion != "" {
		return NewMessageDescriptor("", fmt.Sprintf("Download %s.", latestVersion))
	}
	return NewMessageDescriptor("", "Download the new version.")
}

func updateCurrentMessage() *MessageDescriptor {
	return NewMessageDescriptor(MessageCodeUpdateUpToDate, "You have the latest version.")
}

func updateUnavailableMessage() *MessageDescriptor {
	return NewMessageDescriptor(MessageCodeUpdateCannotCheck, "This build cannot check updates.")
}

func updateErrorMessage(err error) *MessageDescriptor {
	if err == nil {
		return nil
	}
	return NewMessageDescriptor("", err.Error())
}
