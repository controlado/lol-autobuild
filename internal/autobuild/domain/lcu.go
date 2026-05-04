package domain

type ApplyItemSetRequest struct {
	ChampionID int
	Position   Position
	Patch      string
	Blocks     []ApplyItemSetBlock
	DryRun     bool
}

type ApplyItemSetBlock struct {
	Type    string
	ItemIDs []int
}

type RunePage struct {
	PrimaryStyleID  int
	SubStyleID      int
	SelectedPerkIDs []int
}

type ApplyRunePageRequest struct {
	ChampionID int
	Position   Position
	Page       RunePage
	DryRun     bool
}

type ApplySummonerSpellsRequest struct {
	ChampionID int
	Position   Position
	SpellIDs   []int
	KeepFlash  bool
	DryRun     bool
}

type DetectedSelection struct {
	ChampionID   int
	Position     Position
	QueueID      int
	IsAutofilled bool
}

type LCUEvent struct {
	EventType        string
	URI              string
	Source           LCUEventSource
	ConnectionID     int
	ChampSelectPhase string
	GameID           string
}

type LCUEventSource string

const (
	LCUEventSourceStream   LCUEventSource = "event"
	LCUEventSourceSnapshot LCUEventSource = "snapshot"
)

type LCUWatchNoticeKind string

const (
	LCUWatchNoticeConnected            LCUWatchNoticeKind = "connected"
	LCUWatchNoticeReconnecting         LCUWatchNoticeKind = "reconnecting"
	LCUWatchNoticeSnapshotFinalization LCUWatchNoticeKind = "snapshot_finalization"
	LCUWatchNoticeSnapshotWaiting      LCUWatchNoticeKind = "snapshot_waiting"
)

type LCUWatchNotice struct {
	Kind         LCUWatchNoticeKind
	Message      string
	Err          error
	Source       string
	URI          string
	Phase        string
	ConnectionID int
}
