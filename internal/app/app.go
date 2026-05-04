package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild"
)

var ErrUpdateUnavailable = errors.New("update check unavailable")

type (
	ServiceFactory    func(RuntimeConfig) (autobuild.Service, error)
	LCUStatusProvider func(context.Context, RuntimeConfig) LCUStatus
	MessageMapper     func(error) UserMessage
	UpdateChecker     interface {
		CurrentVersion() string
		Check(context.Context) (UpdateCheckResult, error)
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
	UpdateChecker    UpdateChecker
	ConfigStore      ConfigStore
	RuntimeConfig    RuntimeConfig
	MessageFromError MessageMapper
}

type App struct {
	saveMu sync.Mutex // SaveSettings()
	mu     sync.Mutex // App memory

	serviceFactory ServiceFactory
	lcuStatus      LCUStatusProvider
	updateChecker  UpdateChecker
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

	lastErrorMessage string
	lastErrorCode    string
	lastWatchNotice  *WatcherNoticeState
	lastSync         *SyncSummary
	lastSyncAt       time.Time
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
		updateChecker:  opts.UpdateChecker,
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
			Watcher: WatcherState{
				Running:     a.watcherRunning,
				ConfigStale: a.watcherConfigStaleLocked(configSnapshot),
				LastNotice:  cloneWatcherNotice(a.lastWatchNotice),
			},
			Update:        cloneUpdateState(a.updateState),
			SyncRunning:   a.syncRunning,
			LastSync:      cloneSyncSummary(a.lastSync),
			LastError:     a.lastErrorMessage,
			LastErrorCode: a.lastErrorCode,
		}
	)

	if !a.lastSyncAt.IsZero() {
		lastSyncAtCopy := a.lastSyncAt
		state.LastSyncAt = &lastSyncAtCopy
	}

	a.mu.Unlock()

	state.LCU = a.lcuStatus(ctx, configSnapshot)
	return state
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
	a.setLastErrorMessage(UserMessage{})
	a.mu.Unlock()

	return a.State(ctx), UserMessage{}
}

func (a *App) watcherConfigStaleLocked(cfg RuntimeConfig) bool {
	return a.watcherRunning && a.watcherConfigSet && cfg != a.watcherConfig
}

func (a *App) configSnapshot() RuntimeConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) setLastErrorMessage(msg UserMessage) {
	a.lastErrorMessage = msg.Text
	a.lastErrorCode = msg.Code
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
	req := watchRequestFromConfig(cfg)
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
		a.setLastErrorMessage(a.messageFromErr(err))
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
	a.setLastErrorMessage(UserMessage{})

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
		a.setLastErrorMessage(a.messageFromErr(c.Err))
		return
	}

	if c.Result != nil {
		a.lastSync = syncSummaryFromResult(*c.Result)
		a.lastSyncAt = time.Now().UTC()
	}

	a.setLastErrorMessage(UserMessage{})
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
		Message:      notice.Message,
		Source:       notice.Source,
		URI:          notice.URI,
		Phase:        notice.Phase,
		ConnectionID: notice.ConnectionID,
		At:           at,
	}
	if notice.Err != nil {
		state.Error = notice.Err.Error()
	}
	return state
}

func cloneWatcherNotice(notice *WatcherNoticeState) *WatcherNoticeState {
	if notice == nil {
		return nil
	}
	out := *notice
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
		a.setLastErrorMessage(a.messageFromErr(err))
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

	result, err := svc.Sync(ctx, syncRequestFromSettings(cfg.Settings))
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
	a.setLastErrorMessage(UserMessage{})
	return a.cfg, false
}

func (a *App) finishSync(res *autobuild.SyncResult, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.syncRunning = false

	if err != nil {
		a.setLastErrorMessage(a.messageFromErr(err))
		return
	}

	if res == nil {
		a.lastSync = nil
	} else {
		a.lastSync = syncSummaryFromResult(*res)
	}
	a.lastSyncAt = time.Now().UTC()
	a.setLastErrorMessage(UserMessage{})
}

func cloneSyncSummary(res *SyncSummary) *SyncSummary {
	if res == nil {
		return nil
	}

	out := *res
	out.Warnings = append([]string{}, res.Warnings...)
	return &out
}

func syncSummaryFromResult(res autobuild.SyncResult) *SyncSummary {
	return &SyncSummary{
		DetectedChampionID: res.DetectedChampionID,
		DetectedPosition:   res.DetectedPosition,
		DetectedQueueID:    res.DetectedQueueID,
		ItemSetApplied:     res.ItemSetApplied,
		RunePageApplied:    res.RunePageApplied,
		SpellsApplied:      res.SpellsApplied,
		Warnings:           append([]string{}, res.Warnings...),
	}
}

func syncRequestFromSettings(settings Settings) autobuild.SyncRequest {
	return autobuild.SyncRequest{
		Patch:              settings.Patch,
		PatchAdditionsMode: settings.PatchAdditionsMode,
		PatchAdditions:     settings.PatchAdditions,
		LeagueTierPreset:   settings.LeagueTierPreset,
		ApplyItems:         settings.ApplyItems,
		ApplyRunes:         settings.ApplyRunes,
		ApplySpells:        settings.ApplySpells,
		KeepFlash:          settings.KeepFlash,
		DryRun:             settings.DryRun,
	}
}

func watchRequestFromConfig(cfg RuntimeConfig) autobuild.WatchRequest {
	req := syncRequestFromSettings(cfg.Settings)
	return autobuild.WatchRequest{
		Patch:              req.Patch,
		PatchAdditionsMode: req.PatchAdditionsMode,
		PatchAdditions:     req.PatchAdditions,
		LeagueTierPreset:   req.LeagueTierPreset,
		ApplyItems:         req.ApplyItems,
		ApplyRunes:         req.ApplyRunes,
		ApplySpells:        req.ApplySpells,
		KeepFlash:          req.KeepFlash,
		DryRun:             req.DryRun,
		Debounce:           cfg.WatchDebounce,
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
		if newState.LatestVersion != "" {
			newState.Message = fmt.Sprintf("Download %s.", newState.LatestVersion)
		} else {
			newState.Message = "Download the new version."
		}
	case err == nil:
		newState.Status = UpdateStatusCurrent
		newState.Message = "You have the latest version."
	case errors.Is(err, ErrUpdateUnavailable):
		newState.Status = UpdateStatusUnavailable
		newState.Message = "This build cannot check updates."
	default:
		newState.Status = UpdateStatusError
		newState.Message = err.Error()
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
	return out
}
