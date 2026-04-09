package ports

import (
	"context"
	"encoding/json"
	"time"
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

type KeystoneStat struct {
	Rune        int          `json:"rune"`
	RuneType    int          `json:"runeType"`
	WPAOverall  float64      `json:"wpaOverall"`
	Occurrence  int          `json:"occurrence"`
	RuneEffects []RuneEffect `json:"runeEffects"`
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

type CoachlessClient interface {
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
	GetPatches(ctx context.Context, accessToken string) ([]PatchInfo, error)
	GetKeystoneData(ctx context.Context, accessToken string, req KeystoneRequest) ([]KeystoneStat, error)
	GetSummonerSpellStats(ctx context.Context, accessToken string, req SummonerSpellStatsRequest) ([]SummonerSpellStat, error)
	GetItemStats(ctx context.Context, accessToken string, req ItemStatsRequest) ([]ItemStat, error)
}

type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
	Refresh(ctx context.Context) (TokenPair, error)
}

type SecretStore interface {
	ReadTokens(ctx context.Context) (TokenPair, error)
	WriteTokens(ctx context.Context, pair TokenPair) error
	ClearTokens(ctx context.Context) error
}

type ApplyItemSetRequest struct {
	ChampionID int
	Role       string
	Patch      string
	ItemIDs    []int
	DryRun     bool
}

type ApplyRunePageRequest struct {
	ChampionID int
	Role       string
	KeystoneID int
	DryRun     bool
}

type ApplySummonerSpellsRequest struct {
	ChampionID int
	Role       string
	SpellIDs   []int
	DryRun     bool
}

type DetectedSelection struct {
	ChampionID   int
	Role         string
	QueueID      int
	IsAutofilled bool
}

type LCUEvent struct {
	EventType string
	URI       string
	Data      json.RawMessage
}

type LCUClient interface {
	DetectSelection(ctx context.Context) (DetectedSelection, error)
	ApplyItemSet(ctx context.Context, req ApplyItemSetRequest) error
	ApplyRunePage(ctx context.Context, req ApplyRunePageRequest) error
	ApplySummonerSpells(ctx context.Context, req ApplySummonerSpellsRequest) error
	WatchEvents(ctx context.Context, out chan<- LCUEvent) error
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

type RecommendationEngine interface {
	Recommend(input RecommendationInput) Recommendation
}
