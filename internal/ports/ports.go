package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/controlado/lol-autobuild/internal/position"
)

type PatchInfo struct {
	Label      string `json:"label"`
	Major      int    `json:"major"`
	Patch      int    `json:"patch"`
	MatchCount int    `json:"matchCount"`
}

type PatchFilter struct {
	Major          int `json:"major"`
	Patch          int `json:"patch"`
	PatchAdditions int `json:"patchAdditions"`
}

type CommonFilters struct {
	Patch              PatchFilter `json:"patch"`
	ChampionIDs        []int       `json:"championIds"`
	MatchupChampionIDs []int       `json:"matchupChampionIds,omitempty"`
	LeagueTiers        []int       `json:"leagueTiers"`
	Regions            []string    `json:"regions,omitempty"`
	Role               int         `json:"role"`
}

type RuneEffect struct {
	Type  int    `json:"type"`
	Value string `json:"value"`
}

const (
	RuneStylePrecision   = 8000
	RuneStyleDomination  = 8100
	RuneStyleSorcery     = 8200
	RuneStyleInspiration = 8300
	RuneStyleResolve     = 8400
)

const (
	RuneKeystonePressTheAttack    = 8005
	RuneKeystoneLethalTempo       = 8008
	RuneKeystoneFleetFootwork     = 8021
	RuneKeystoneConqueror         = 8010
	RuneKeystoneElectrocute       = 8112
	RuneKeystoneDarkHarvest       = 8128
	RuneKeystoneHailOfBlades      = 9923
	RuneKeystoneSummonAery        = 8214
	RuneKeystoneArcaneComet       = 8229
	RuneKeystoneStormraidersSurge = 8230
	RuneKeystoneDeathfireTouch    = 8992
	RuneKeystoneGraspOfTheUndying = 8437
	RuneKeystoneAftershock        = 8439
	RuneKeystoneGuardian          = 8465
	RuneKeystoneGlacialAugment    = 8351
	RuneKeystoneUnsealedSpellbook = 8360
	RuneKeystoneFirstStrike       = 8369
)

type KeystoneStat struct {
	Rune        int          `json:"rune"`
	RuneType    int          `json:"runeType"`
	WPAOverall  float64      `json:"wpaOverall"`
	Occurrence  int          `json:"occurrence"`
	RuneEffects []RuneEffect `json:"runeEffects"`
}

type RuneStat struct {
	Rune        int          `json:"rune"`
	RuneType    int          `json:"runeType"`
	WPAOverall  float64      `json:"wpaOverall"`
	Occurrence  int          `json:"occurrence"`
	RuneEffects []RuneEffect `json:"runeEffects"`
}

type RuneTreePlaycount struct {
	Tree       int     `json:"tree"`
	Occurrence float64 `json:"occurrence"`
}

type RuneStatsByRow struct {
	RowOnes   []RuneStat `json:"rowOnes"`
	RowTwos   []RuneStat `json:"rowTwos"`
	RowThrees []RuneStat `json:"rowThrees"`
}

type ShardStats struct {
	Offense []RuneStat `json:"offense"`
	Flex    []RuneStat `json:"flex"`
	Defense []RuneStat `json:"defense"`
}

type SummonerSpellStat struct {
	SummonerSpell      int     `json:"summonerSpell"`
	WPAOverall         float64 `json:"wpaOverall"`
	Occurrence         int     `json:"occurrence"`
	AverageCasts       float64 `json:"averageCasts"`
	OccurrenceRelative float64 `json:"occurrenceRelative"`
	WinrateExpected    float64 `json:"winrateExpected"`
	WinrateObserved    float64 `json:"winrateObserved"`
}

type ItemStat struct {
	ItemID                 int     `json:"itemId"`
	WPAStandalone          float64 `json:"wpaStandalone"`
	WPAOverall             float64 `json:"wpaOverall"`
	Occurrence             int     `json:"occurrence"`
	OccurrenceRelative     float64 `json:"occurrenceRelative"`
	WinrateExpected        float64 `json:"winrateExpected"`
	WinrateObserved        float64 `json:"winrateObserved"`
	AveragePurchaseTime    float64 `json:"averagePurchaseTime"`
	Bias                   float64 `json:"bias"`
	GoodPurchaseSituations []any   `json:"goodPurchaseSituations"`
}

type KeystoneRequest struct {
	CommonFilters CommonFilters `json:"commonFilters"`
}

type SecondaryTreePlaycountRequest struct {
	CommonFilters CommonFilters `json:"commonFilters"`
	Tree          int           `json:"tree"`
	Keystone      int           `json:"keystone"`
}

type RuneStatsRequest struct {
	CommonFilters CommonFilters `json:"commonFilters"`
	Keystone      int           `json:"keystone"`
	MainTree      int           `json:"mainTree"`
	TreeToLoad    int           `json:"treeToLoad"`
}

type ShardStatsRequest struct {
	CommonFilters CommonFilters `json:"commonFilters"`
	Keystone      int           `json:"keystone"`
}

type SummonerSpellStatsRequest struct {
	CommonFilters CommonFilters `json:"commonFilters"`
	PairedSpell   *int          `json:"pairedSpell"`
}

type ItemStatsRequest struct {
	CommonFilters         CommonFilters `json:"commonFilters"`
	ItemSlots             []int         `json:"itemSlots,omitempty"`
	ItemType              int           `json:"itemType"`
	Keystone              *int          `json:"keystone"`
	StarterID             *int          `json:"starterId"`
	FirstPurchaseID       *int          `json:"firstPurchaseId"`
	FirstLegendaryID      *int          `json:"firstLegendaryId"`
	SecondLegendaryID     *int          `json:"secondLegendaryId"`
	LoadFirstEpicPurchase bool          `json:"loadFirstEpicPurchase"`
	IncludeSupportItems   bool          `json:"includeSupportItems"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type TokenClaims struct {
	Exp             int64  `json:"exp"`
	IsSubscribedRaw string `json:"isSubscribed"`
}

func (tc *TokenClaims) IsSubscribed() bool {
	return tc.IsSubscribedRaw == "1"
}

type CoachlessClient interface {
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
	GetPatches(ctx context.Context, accessToken string) ([]PatchInfo, error)
	GetKeystoneData(ctx context.Context, accessToken string, req KeystoneRequest) ([]KeystoneStat, error)
	GetSecondaryTreePlaycount(ctx context.Context, accessToken string, req SecondaryTreePlaycountRequest) ([]RuneTreePlaycount, error)
	GetRuneStatsForKeystoneAndTree(ctx context.Context, accessToken string, req RuneStatsRequest) (RuneStatsByRow, error)
	GetShardStatsForKeystoneAndTree(ctx context.Context, accessToken string, req ShardStatsRequest) (ShardStats, error)
	GetSummonerSpellStats(ctx context.Context, accessToken string, req SummonerSpellStatsRequest) ([]SummonerSpellStat, error)
	GetItemStats(ctx context.Context, accessToken string, req ItemStatsRequest) ([]ItemStat, error)
}

type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
	Refresh(ctx context.Context) (TokenPair, error)
	Claims(ctx context.Context) (TokenClaims, error)
}

type SecretStore interface {
	ReadTokens(ctx context.Context) (TokenPair, error)
	WriteTokens(ctx context.Context, pair TokenPair) error
	ClearTokens(ctx context.Context) error
}

type ApplyItemSetRequest struct {
	ChampionID int
	Position   position.Position
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
	Position   position.Position
	Page       RunePage
	DryRun     bool
}

type ApplySummonerSpellsRequest struct {
	ChampionID int
	Position   position.Position
	SpellIDs   []int
	KeepFlash  bool
	DryRun     bool
}

type DetectedSelection struct {
	ChampionID   int
	Position     position.Position
	QueueID      int
	IsAutofilled bool
}

type LCUEvent struct {
	EventType    string
	URI          string
	Data         json.RawMessage
	Source       LCUEventSource
	ConnectionID int
}

type LCUClient interface {
	DetectSelection(ctx context.Context) (DetectedSelection, error)
	ApplyItemSet(ctx context.Context, req ApplyItemSetRequest) error
	ApplyRunePage(ctx context.Context, req ApplyRunePageRequest) error
	ApplySummonerSpells(ctx context.Context, req ApplySummonerSpellsRequest) error
	WatchEventsWithNotices(ctx context.Context, out chan<- LCUEvent, notices chan<- LCUWatchNotice) error
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

type RecommendationInput struct {
	KeystoneStats []KeystoneStat
	SpellStats    []SummonerSpellStat
	ItemStats     []ItemStat
	MinOccurrence int
	TopItems      int
	TopSpells     int
}

type Recommendation struct {
	Keystone       *KeystoneStat
	SummonerSpells []SummonerSpellStat
	Items          []ItemStat
	Warnings       []string
}

type RunePageRecommendationInput struct {
	Keystone               KeystoneStat
	SecondaryTreePlaycount []RuneTreePlaycount
	PrimaryRunes           RuneStatsByRow
	SecondaryRunes         RuneStatsByRow
	Shards                 ShardStats
	MinOccurrence          int
}

type RunePageRecommendation struct {
	Page     *RunePage
	Warnings []string
}

type RecommendationEngine interface {
	Recommend(input RecommendationInput) Recommendation
	RecommendRunePage(input RunePageRecommendationInput) RunePageRecommendation
}
