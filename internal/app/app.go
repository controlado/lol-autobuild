package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild"
	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

var ErrUpdateUnavailable = errors.New("update check unavailable")

const lcuStateRefreshTimeout = 750 * time.Millisecond

type (
	ServiceFactory      func(RuntimeConfig) (autobuild.Service, error)
	LCUStatusProvider   func(context.Context, RuntimeConfig) LCUStatus
	ChampSelectProvider func(context.Context, RuntimeConfig) (domain.ChampSelectState, error)
	MessageMapper       func(error) UserMessage
	UpdateChecker       interface {
		CurrentVersion() string
		Check(context.Context) (UpdateCheckResult, error)
	}
	CoachlessAuthSession interface {
		Status(context.Context) CoachlessAuthState
		Login(context.Context) error
		Logout(context.Context) error
	}
)

type UpdateCheckResult struct {
	CurrentVersion string
	LatestVersion  string
	DownloadURL    string
	Available      bool
}

type Options struct {
	ServiceFactory   ServiceFactory
	LCUStatus        LCUStatusProvider
	ChampSelect      ChampSelectProvider
	UpdateChecker    UpdateChecker
	CoachlessAuth    CoachlessAuthSession
	ConfigStore      ConfigStore
	RuntimeConfig    RuntimeConfig
	MessageFromError MessageMapper
}

type App struct {
	saveMu sync.Mutex // SaveSettings()
	authMu sync.Mutex // Coachless auth actions
	mu     sync.Mutex // App memory

	serviceFactory ServiceFactory
	lcuStatus      LCUStatusProvider
	champSelect    ChampSelectProvider
	updateChecker  UpdateChecker
	coachlessAuth  CoachlessAuthSession
	configStore    ConfigStore
	messageFromErr MessageMapper
	cfg            RuntimeConfig

	syncRunning      bool
	watcherStarting  bool
	watcherRunning   bool
	watcherID        int
	cancelWatcher    context.CancelFunc
	watcherConfig    RuntimeConfig
	watcherConfigSet bool
	updateState      UpdateState

	lastError       *MessageDescriptor
	lastWatchNotice *WatcherNoticeState
	lastSync        *SyncSummary
	lastSyncAt      time.Time

	champSelectSessionKey    string
	enemyChampions           []ChampionRef
	selectedEnemyChampionIDs []int
}

func New(opts Options) (*App, error) {
	if opts.ServiceFactory == nil || opts.LCUStatus == nil || opts.UpdateChecker == nil || opts.ConfigStore == nil {
		return nil, fmt.Errorf("serviceFactory, lcuStatus, updateChecker, configStore cannot be nil")
	}
	if opts.MessageFromError == nil {
		opts.MessageFromError = userMessageFromErr
	}

	return &App{
		serviceFactory: opts.ServiceFactory,
		lcuStatus:      opts.LCUStatus,
		champSelect:    opts.ChampSelect,
		updateChecker:  opts.UpdateChecker,
		coachlessAuth:  opts.CoachlessAuth,
		configStore:    opts.ConfigStore,
		messageFromErr: opts.MessageFromError,
		cfg:            normalizeConfig(opts.RuntimeConfig),
		updateState: UpdateState{
			Status:         UpdateStatusIdle,
			CurrentVersion: opts.UpdateChecker.CurrentVersion(),
		},
	}, nil
}

func (a *App) State(ctx context.Context) ViewState {
	a.mu.Lock()

	var (
		configSnapshot = a.cfg
		state          = ViewState{
			Settings: configSnapshot.Settings,
			CoachlessAuth: CoachlessAuthState{
				Status: CoachlessAuthStatusMissing,
				Plan:   CoachlessAuthPlanUnknown,
			},
			Watcher: WatcherState{
				Running:     a.watcherRunning,
				ConfigStale: a.watcherConfigStaleLocked(configSnapshot),
				LastNotice:  cloneWatcherNotice(a.lastWatchNotice),
			},
			ChampSelect: ChampSelectState{
				EnemyChampions:           []ChampionRef{},
				SelectedEnemyChampionIDs: []int{},
			},
			Update:      cloneUpdateState(a.updateState),
			SyncRunning: a.syncRunning,
			LastSync:    cloneSyncSummary(a.lastSync),
			LastError:   cloneMessageDescriptor(a.lastError),
		}
	)

	if !a.lastSyncAt.IsZero() {
		lastSyncAtCopy := a.lastSyncAt
		state.LastSyncAt = &lastSyncAtCopy
	}

	a.mu.Unlock()

	if a.coachlessAuth != nil {
		state.CoachlessAuth = cloneCoachlessAuthState(a.coachlessAuth.Status(ctx))
	}
	state.ChampSelect = a.refreshChampSelect(ctx, configSnapshot)
	statusCtx, cancel := context.WithTimeout(ctx, lcuStateRefreshTimeout)
	state.LCU = cloneLCUStatus(a.lcuStatus(statusCtx, configSnapshot))
	cancel()

	return state
}

func cloneLCUStatus(status LCUStatus) LCUStatus {
	out := status
	out.Message = cloneMessageDescriptor(status.Message)
	return out
}

func cloneCoachlessAuthState(state CoachlessAuthState) CoachlessAuthState {
	out := state
	if state.ExpiresAt != nil {
		expiresAt := *state.ExpiresAt
		out.ExpiresAt = &expiresAt
	}
	out.Message = cloneMessageDescriptor(state.Message)
	return out
}

func (a *App) refreshChampSelect(ctx context.Context, cfg RuntimeConfig) ChampSelectState {
	if state, ok := a.observeChampSelectFromProvider(ctx, cfg); ok {
		return state
	}

	return a.clearVisibleEnemyChampions()
}

func (a *App) observeChampSelectFromProvider(ctx context.Context, cfg RuntimeConfig) (ChampSelectState, bool) {
	if a.champSelect == nil || !cfg.Settings.LCUEnabled {
		return ChampSelectState{}, false
	}

	champSelectCtx, cancel := context.WithTimeout(ctx, lcuStateRefreshTimeout)
	defer cancel()

	state, err := a.champSelect(champSelectCtx, cfg)
	if err != nil {
		return ChampSelectState{}, false
	}

	return a.observeChampSelectState(state), true
}

func (a *App) clearVisibleEnemyChampions() ChampSelectState {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.enemyChampions = nil
	return a.champSelectStateLocked()
}

func (a *App) observeChampSelectState(state domain.ChampSelectState) ChampSelectState {
	a.mu.Lock()
	defer a.mu.Unlock()

	if state.SessionKey == "" {
		a.enemyChampions = nil
		return a.champSelectStateLocked()
	}

	if a.champSelectSessionKey != state.SessionKey {
		a.champSelectSessionKey = state.SessionKey
		a.selectedEnemyChampionIDs = nil
	}

	a.enemyChampions = appChampionRefsFromDomain(state.EnemyChampions)
	a.selectedEnemyChampionIDs = selectedEnemyChampionIDs(a.selectedEnemyChampionIDs, a.enemyChampions)
	return a.champSelectStateLocked()
}

func (a *App) champSelectStateLocked() ChampSelectState {
	enemyChampions := slices.Clone(a.enemyChampions)
	if len(enemyChampions) == 0 {
		enemyChampions = []ChampionRef{}
	}

	return ChampSelectState{
		EnemyChampions:           enemyChampions,
		SelectedEnemyChampionIDs: selectedEnemyChampionIDs(a.selectedEnemyChampionIDs, a.enemyChampions),
	}
}

func appChampionRefsFromDomain(in []domain.ChampionRef) []ChampionRef {
	out := make([]ChampionRef, 0, len(in))
	for _, champion := range in {
		if champion.ID <= 0 {
			continue
		}
		out = append(out, ChampionRef{
			ID:   champion.ID,
			Name: strings.TrimSpace(champion.Name),
		})
	}
	return out
}

func selectedEnemyChampionIDs(requested []int, enemies []ChampionRef) []int {
	selected := domain.MatchupChampionIDsForRoster(requested, domainChampionRefsFromApp(enemies), domain.MaxMatchupChampionIDs)
	if len(selected) == 0 {
		return []int{}
	}
	return selected
}

func domainChampionRefsFromApp(in []ChampionRef) []domain.ChampionRef {
	out := make([]domain.ChampionRef, 0, len(in))
	for _, champion := range in {
		if champion.ID <= 0 {
			continue
		}
		out = append(out, domain.ChampionRef{ID: champion.ID, Name: champion.Name})
	}
	return out
}

func (a *App) SetEnemyChampionSelection(ctx context.Context, championIDs []int) EnemyChampionSelectionState {
	cfg := a.configSnapshot()
	if a.champSelect == nil || !cfg.Settings.LCUEnabled {
		_ = a.clearVisibleEnemyChampions()
	} else {
		_, _ = a.observeChampSelectFromProvider(ctx, cfg)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.selectedEnemyChampionIDs = selectedEnemyChampionIDs(championIDs, a.enemyChampions)
	selected := slices.Clone(a.selectedEnemyChampionIDs)
	if len(selected) == 0 {
		selected = []int{}
	}

	return EnemyChampionSelectionState{
		SelectedEnemyChampionIDs: selected,
	}
}

func (a *App) selectedEnemyChampionIDsForRequest(ctx context.Context, cfg RuntimeConfig) []int {
	if a.champSelect == nil || !cfg.Settings.LCUEnabled {
		return []int{}
	}

	if observed, ok := a.observeChampSelectFromProvider(ctx, cfg); ok {
		return slices.Clone(observed.SelectedEnemyChampionIDs)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.selectedEnemyChampionIDs)
}

func (a *App) SaveSettings(ctx context.Context, settings Settings) (ViewState, UserMessage) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	cfg := a.configSnapshot()
	cfg.Settings = normalizeSettings(settings)

	if err := a.configStore.Save(cfg); err != nil {
		return ViewState{}, a.messageFromErr(err)
	}

	a.mu.Lock()
	a.cfg = cfg
	a.setLastErrorLocked(UserMessage{})
	a.mu.Unlock()

	return a.State(ctx), UserMessage{}
}

func (a *App) LoginCoachlessAuth(ctx context.Context) (ViewState, UserMessage) {
	return a.runCoachlessAuthAction(ctx, func() error { return a.coachlessAuth.Login(ctx) })
}

func (a *App) LogoutCoachlessAuth(ctx context.Context) (ViewState, UserMessage) {
	return a.runCoachlessAuthAction(ctx, func() error { return a.coachlessAuth.Logout(ctx) })
}

func (a *App) runCoachlessAuthAction(ctx context.Context, action func() error) (ViewState, UserMessage) {
	var msg = UserMessage{}

	a.authMu.Lock()
	defer a.authMu.Unlock()

	if a.coachlessAuth == nil {
		msg = coachlessAuthUnavailableMessage()
		a.setLastError(msg)
		return ViewState{}, msg
	}

	if err := action(); err != nil {
		msg = a.messageFromErr(err)
		a.setLastError(msg)
		return ViewState{}, msg
	}

	a.setLastError(msg)
	return a.State(ctx), msg
}

func (a *App) watcherConfigStaleLocked(cfg RuntimeConfig) bool {
	return a.watcherRunning && a.watcherConfigSet && cfg != a.watcherConfig
}

func (a *App) configSnapshot() RuntimeConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) setLastError(msg UserMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.setLastErrorLocked(msg)
}

func (a *App) setLastErrorLocked(msg UserMessage) {
	a.lastError = msg.Descriptor()
}

func normalizeConfig(cfg RuntimeConfig) RuntimeConfig {
	cfg.Settings = normalizeSettings(cfg.Settings)
	return cfg
}

func normalizeSettings(settings Settings) Settings {
	patchAdditionsMode := strings.TrimSpace(settings.PatchAdditionsMode)
	if patchAdditionsMode == "" {
		patchAdditionsMode = autobuild.PatchAdditionsModeAuto
	}

	patchAdditions := settings.PatchAdditions
	if patchAdditionsMode == autobuild.PatchAdditionsModeAuto && patchAdditions == 0 {
		patchAdditions = autobuild.PatchAdditionsDefault
	}

	leagueTierPreset := strings.TrimSpace(settings.LeagueTierPreset)
	if leagueTierPreset == "" {
		leagueTierPreset = autobuild.LeagueTierPresetDefault
	}

	return Settings{
		Patch:              strings.TrimSpace(settings.Patch),
		PatchAdditionsMode: patchAdditionsMode,
		PatchAdditions:     patchAdditions,
		LeagueTierPreset:   leagueTierPreset,
		ApplyItems:         settings.ApplyItems,
		ApplyRunes:         settings.ApplyRunes,
		ApplySpells:        settings.ApplySpells,
		KeepFlash:          settings.KeepFlash,
		DryRun:             settings.DryRun,
		LCUEnabled:         settings.LCUEnabled,
	}
}

func (a *App) StartWatcher(ctx context.Context) (ViewState, UserMessage) {
	cfg, watcherID, ok := a.reserveWatcherStart()
	if !ok {
		return a.State(ctx), watcherPreStartFailedMessage()
	}

	svc, err := a.serviceFactory(cfg)
	if err != nil {
		a.releaseWatcher(watcherID, err)
		return ViewState{}, a.messageFromErr(err)
	}

	if err := svc.EnsureCoachlessAuth(ctx); err != nil {
		a.releaseWatcher(watcherID, err)
		return ViewState{}, a.messageFromErr(err)
	}

	watcherCtx, ok := a.startReservedWatcher(watcherID, cfg)
	if !ok {
		a.releaseWatcher(watcherID, nil)
		return a.State(ctx), watcherStartFailedMessage()
	}

	go a.runWatcher(watcherCtx, watcherID, svc, cfg)

	return a.State(ctx), UserMessage{}
}

func (a *App) runWatcher(ctx context.Context, watcherID int, svc autobuild.Service, cfg RuntimeConfig) {
	req := watchRequestFromConfig(cfg, func() []int { return a.selectedEnemyChampionIDsForRequest(ctx, cfg) })
	req.OnCycle = func(c autobuild.WatchCycle) { a.observeWatchCycle(watcherID, c) }
	req.OnNotice = func(n autobuild.WatchNotice) { a.observeWatchNotice(watcherID, n) }

	err := svc.Watch(ctx, req)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherID != watcherID {
		return
	}

	a.cancelWatcher = nil
	a.watcherRunning = false
	a.watcherConfig = RuntimeConfig{}
	a.watcherConfigSet = false

	if err != nil && ctx.Err() == nil {
		a.setLastErrorLocked(a.messageFromErr(err))
	}
}

func (a *App) reserveWatcherStart() (cfg RuntimeConfig, watcherID int, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherStarting || a.watcherRunning {
		return RuntimeConfig{}, 0, false
	}

	a.watcherID++
	a.watcherStarting = true
	a.setLastErrorLocked(UserMessage{})

	return a.cfg, a.watcherID, true
}

func (a *App) startReservedWatcher(watcherID int, cfg RuntimeConfig) (watcherCtx context.Context, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherID != watcherID || !a.watcherStarting {
		return nil, false
	}

	watcherCtx, a.cancelWatcher = context.WithCancel(context.Background())
	a.watcherStarting = false
	a.watcherRunning = true
	a.watcherConfig = cfg
	a.watcherConfigSet = true

	return watcherCtx, true
}

func (a *App) observeWatchCycle(watchID int, c autobuild.WatchCycle) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.watcherRunning || watchID != a.watcherID {
		return
	}

	if c.Err != nil {
		a.setLastErrorLocked(a.messageFromErr(c.Err))
		return
	}

	if c.Result != nil {
		a.lastSync = syncSummaryFromResult(*c.Result)
		a.lastSyncAt = time.Now().UTC()
	}

	a.setLastErrorLocked(UserMessage{})
}

func (a *App) observeWatchNotice(watchID int, notice autobuild.WatchNotice) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.watcherRunning || watchID != a.watcherID {
		return
	}

	noticeState := watcherNoticeStateFromNotice(notice, time.Now().UTC())
	a.lastWatchNotice = &noticeState
}

func watcherNoticeStateFromNotice(notice autobuild.WatchNotice, at time.Time) WatcherNoticeState {
	state := WatcherNoticeState{
		Kind:         string(notice.Kind),
		Message:      NewMessageDescriptor(watchNoticeMessageKey(notice.Kind), notice.Message),
		Source:       notice.Source,
		URI:          notice.URI,
		Phase:        notice.Phase,
		ConnectionID: notice.ConnectionID,
		At:           at,
	}
	if notice.Err != nil {
		state.Error = NewMessageDescriptor("", notice.Err.Error())
	}
	return state
}

func watchNoticeMessageKey(kind autobuild.WatchNoticeKind) string {
	switch kind {
	case autobuild.WatchNoticeConnected:
		return MessageCodeWatchNoticeConnected
	case autobuild.WatchNoticeReconnecting:
		return MessageCodeWatchNoticeReconnecting
	case autobuild.WatchNoticeSnapshotFinalization:
		return MessageCodeWatchNoticeSnapshotFinalization
	case autobuild.WatchNoticeSnapshotWaiting:
		return MessageCodeWatchNoticeSnapshotWaiting
	default:
		return ""
	}
}

func cloneWatcherNotice(notice *WatcherNoticeState) *WatcherNoticeState {
	if notice == nil {
		return nil
	}
	out := *notice
	out.Message = cloneMessageDescriptor(notice.Message)
	out.Error = cloneMessageDescriptor(notice.Error)
	return &out
}

func cloneMessageDescriptor(descriptor *MessageDescriptor) *MessageDescriptor {
	if descriptor == nil {
		return nil
	}
	out := *descriptor
	return &out
}

func (a *App) StopWatcher(ctx context.Context) ViewState {
	a.mu.Lock()
	cancel := a.cancelWatcher
	a.cancelWatcher = nil
	a.watcherStarting = false
	a.watcherRunning = false
	a.watcherConfig = RuntimeConfig{}
	a.watcherConfigSet = false
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	return a.State(ctx)
}

func (a *App) releaseWatcher(watcherID int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherID != watcherID {
		return
	}

	if a.cancelWatcher != nil {
		a.cancelWatcher()
	}

	a.cancelWatcher = nil
	a.watcherStarting = false
	a.watcherRunning = false
	a.watcherConfig = RuntimeConfig{}
	a.watcherConfigSet = false

	if err != nil {
		a.setLastErrorLocked(a.messageFromErr(err))
	}
}

func (a *App) RunSync(ctx context.Context) (ViewState, UserMessage) {
	cfg, alreadySyncing := a.beginSync()
	if alreadySyncing {
		return ViewState{}, syncAlreadyRunningMessage()
	}

	svc, err := a.serviceFactory(cfg)
	if err != nil {
		a.finishSync(nil, err)
		return ViewState{}, a.messageFromErr(err)
	}

	result, err := svc.Sync(ctx, syncRequestFromSettings(cfg.Settings, a.selectedEnemyChampionIDsForRequest(ctx, cfg)))
	a.finishSync(&result, err)
	if err != nil {
		return ViewState{}, a.messageFromErr(err)
	}

	return a.State(ctx), UserMessage{}
}

func (a *App) beginSync() (configSnapshot RuntimeConfig, alreadySyncing bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.syncRunning {
		return RuntimeConfig{}, true
	}

	a.syncRunning = true
	a.setLastErrorLocked(UserMessage{})
	return a.cfg, false
}

func (a *App) finishSync(res *autobuild.SyncResult, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.syncRunning = false

	if err != nil {
		a.setLastErrorLocked(a.messageFromErr(err))
		return
	}

	if res == nil {
		a.lastSync = nil
	} else {
		a.lastSync = syncSummaryFromResult(*res)
	}
	a.lastSyncAt = time.Now().UTC()
	a.setLastErrorLocked(UserMessage{})
}

func cloneSyncSummary(res *SyncSummary) *SyncSummary {
	if res == nil {
		return nil
	}

	out := *res
	out.Warnings = slices.Clone(res.Warnings)
	return &out
}

func syncSummaryFromResult(res autobuild.SyncResult) *SyncSummary {
	return &SyncSummary{
		DetectedChampionID:   res.DetectedChampionID,
		DetectedChampionName: res.DetectedChampionName,
		DetectedPosition:     res.DetectedPosition,
		DetectedQueueID:      res.DetectedQueueID,
		ItemSetApplied:       res.ItemSetApplied,
		RunePageApplied:      res.RunePageApplied,
		SpellsApplied:        res.SpellsApplied,
		Warnings:             syncWarningDescriptorsFromText(res.Warnings),
	}
}

func syncWarningDescriptorsFromText(warnings []string) []MessageDescriptor {
	if len(warnings) == 0 {
		return []MessageDescriptor{}
	}

	out := make([]MessageDescriptor, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, syncWarningDescriptorFromText(warning))
	}
	return out
}

func syncWarningDescriptorFromText(warning string) MessageDescriptor {
	switch warning {
	case autobuild.RunePageLimitReachedWarning:
		return MessageDescriptor{Key: MessageCodeSyncRunePageLimitReached, Fallback: warning}
	default:
		return MessageDescriptor{Fallback: warning}
	}
}

func syncRequestFromSettings(settings Settings, matchupChampionIDs []int) autobuild.SyncRequest {
	return autobuild.SyncRequest{
		Patch:              settings.Patch,
		PatchAdditionsMode: settings.PatchAdditionsMode,
		PatchAdditions:     settings.PatchAdditions,
		LeagueTierPreset:   settings.LeagueTierPreset,
		MatchupChampionIDs: slices.Clone(matchupChampionIDs),
		ApplyItems:         settings.ApplyItems,
		ApplyRunes:         settings.ApplyRunes,
		ApplySpells:        settings.ApplySpells,
		KeepFlash:          settings.KeepFlash,
		DryRun:             settings.DryRun,
	}
}

func watchRequestFromConfig(cfg RuntimeConfig, selectedMatchupChampionIDs func() []int) autobuild.WatchRequest {
	req := syncRequestFromSettings(cfg.Settings, nil)
	return autobuild.WatchRequest{
		Patch:                      req.Patch,
		PatchAdditionsMode:         req.PatchAdditionsMode,
		PatchAdditions:             req.PatchAdditions,
		LeagueTierPreset:           req.LeagueTierPreset,
		SelectedMatchupChampionIDs: selectedMatchupChampionIDs,
		ApplyItems:                 req.ApplyItems,
		ApplyRunes:                 req.ApplyRunes,
		ApplySpells:                req.ApplySpells,
		KeepFlash:                  req.KeepFlash,
		DryRun:                     req.DryRun,
		Debounce:                   cfg.WatchDebounce,
	}
}

func (a *App) CheckUpdates(ctx context.Context) (ViewState, UserMessage) {
	if !a.beginUpdateCheck() {
		return a.State(ctx), UserMessage{}
	}

	result, err := a.updateChecker.Check(ctx)
	a.finishUpdateCheck(result, err)

	return a.State(ctx), UserMessage{}
}

func (a *App) beginUpdateCheck() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.updateState.Status == UpdateStatusChecking {
		return false
	}

	currentVersion := strings.TrimSpace(a.updateChecker.CurrentVersion())
	if currentVersion == "" {
		currentVersion = a.updateState.CurrentVersion
	}

	a.updateState = UpdateState{
		Status:         UpdateStatusChecking,
		CurrentVersion: currentVersion,
	}

	return true
}

func (a *App) finishUpdateCheck(result UpdateCheckResult, err error) {
	checkedAt := time.Now().UTC()

	currentVersion := strings.TrimSpace(result.CurrentVersion)
	if currentVersion == "" {
		currentVersion = strings.TrimSpace(a.updateChecker.CurrentVersion())
	}

	newState := UpdateState{
		CurrentVersion: currentVersion,
		LatestVersion:  strings.TrimSpace(result.LatestVersion),
		DownloadURL:    strings.TrimSpace(result.DownloadURL),
		CheckedAt:      &checkedAt,
	}

	switch {
	case err == nil && result.Available:
		newState.Status = UpdateStatusAvailable
		newState.Message = updateAvailableMessage(newState.LatestVersion)
	case err == nil:
		newState.Status = UpdateStatusCurrent
		newState.Message = updateCurrentMessage()
	case errors.Is(err, ErrUpdateUnavailable):
		newState.Status = UpdateStatusUnavailable
		newState.Message = updateUnavailableMessage()
	default:
		newState.Status = UpdateStatusError
		newState.Message = updateErrorMessage(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.updateState = newState
}

func cloneUpdateState(state UpdateState) UpdateState {
	out := state
	if state.CheckedAt != nil {
		checkedAt := *state.CheckedAt
		out.CheckedAt = &checkedAt
	}
	out.Message = cloneMessageDescriptor(state.Message)
	return out
}
