package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/internal/update"
	"github.com/controlado/lol-autobuild/pkg/lolautobuild"
)

type (
	ServiceFactory func(config.Config) (lolautobuild.Service, error)
	StatusChecker  func(context.Context, config.Config) lcu.ConnectionStatus
	UpdateChecker  interface {
		CurrentVersion() string
		Check(context.Context) (update.Result, error)
	}
)

type App struct {
	saveMu sync.Mutex // SaveSettings()
	mu     sync.Mutex // App memory

	serviceFactory ServiceFactory
	lcuStatus      StatusChecker
	updateChecker  UpdateChecker
	configStore    ConfigStore
	cfg            config.Config

	syncRunning     bool
	watcherStarting bool
	watcherRunning  bool
	watcherID       int
	cancelWatcher   context.CancelFunc
	updateState     UpdateState

	lastErrorMessage string
	lastSync         *lolautobuild.SyncResult
	lastSyncAt       time.Time
}

func New(serviceFactory ServiceFactory, statusChecker StatusChecker, updateChecker UpdateChecker, configStore ConfigStore, cfg config.Config) (*App, error) {
	if serviceFactory == nil || statusChecker == nil || updateChecker == nil || configStore == nil {
		return nil, fmt.Errorf("serviceFactory, statusChecker, updateChecker, configStore cannot be nil")
	}

	return &App{
		serviceFactory: serviceFactory,
		lcuStatus:      statusChecker,
		updateChecker:  updateChecker,
		configStore:    configStore,
		cfg:            cfg,
		updateState: UpdateState{
			Status:         UpdateStatusIdle,
			CurrentVersion: updateChecker.CurrentVersion(),
		},
	}, nil
}

func (a *App) State(ctx context.Context) State {
	var (
		cfg        = a.configSnapshot()
		lastSyncAt = a.lastSyncAtSnapshot()
		status     = a.lcuStatus(ctx, cfg)
	)

	a.mu.Lock()
	defer a.mu.Unlock()

	return State{
		Settings:    settingsFromConfig(cfg),
		LCU:         status,
		Watcher:     WatcherState{Running: a.watcherRunning},
		Update:      cloneUpdateState(a.updateState),
		SyncRunning: a.syncRunning,
		LastSync:    cloneSyncResult(a.lastSync),
		LastSyncAt:  lastSyncAt,
		LastError:   a.lastErrorMessage,
	}
}

func (a *App) SaveSettings(ctx context.Context, settings Settings) (State, string) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	cfg := a.configSnapshot()
	applySettings(&cfg, settings)

	if err := a.configStore.Save(cfg); err != nil {
		return State{}, messageFromErr(err)
	}

	a.mu.Lock()
	a.cfg = cfg
	a.lastErrorMessage = messageFromErr(nil)
	watcherRunning := a.watcherRunning
	a.mu.Unlock()

	if watcherRunning {
		a.StopWatcher(ctx)
		return a.StartWatcher(ctx)
	}

	return a.State(ctx), messageFromErr(nil)
}

func (a *App) lastSyncAtSnapshot() *time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lastSyncAt.IsZero() {
		return nil
	}

	lastSyncAtCopy := a.lastSyncAt
	return &lastSyncAtCopy
}

func (a *App) configSnapshot() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func settingsFromConfig(cfg config.Config) Settings {
	return Settings{
		Patch:       cfg.Sync.Patch,
		ApplyItems:  cfg.Sync.ApplyItems,
		ApplyRunes:  cfg.Sync.ApplyRunes,
		ApplySpells: cfg.Sync.ApplySpells,
		DryRun:      cfg.Sync.DryRun,
		LCUEnabled:  cfg.LCU.Enabled,
	}
}

func applySettings(cfg *config.Config, settings Settings) {
	cfg.Sync = config.SyncConfig{
		Patch:       strings.TrimSpace(settings.Patch),
		ApplyItems:  settings.ApplyItems,
		ApplyRunes:  settings.ApplyRunes,
		ApplySpells: settings.ApplySpells,
		DryRun:      settings.DryRun,
	}
	cfg.LCU.Enabled = settings.LCUEnabled
}

func (a *App) StartWatcher(ctx context.Context) (State, string) {
	cfg, watcherID, ok := a.reserveWatcherStart()
	if !ok {
		return a.State(ctx), "Watcher pre-start failed."
	}

	svc, err := a.serviceFactory(cfg)
	if err != nil {
		a.releaseWatcher(watcherID, err)
		return State{}, messageFromErr(err)
	}

	if err := svc.EnsureCoachlessAuth(ctx); err != nil {
		a.releaseWatcher(watcherID, err)
		return State{}, "Coachless login is missing."
	}

	watcherCtx, ok := a.startReservedWatcher(watcherID)
	if !ok {
		a.releaseWatcher(watcherID, nil)
		return a.State(ctx), "Watcher start failed."
	}

	go a.runWatcher(watcherCtx, watcherID, svc, cfg)

	return a.State(ctx), ""
}

func (a *App) runWatcher(ctx context.Context, watcherID int, svc lolautobuild.Service, cfg config.Config) {
	err := svc.Watch(ctx, lolautobuild.WatchRequest{
		Patch:       cfg.Sync.Patch,
		ApplyItems:  cfg.Sync.ApplyItems,
		ApplyRunes:  cfg.Sync.ApplyRunes,
		ApplySpells: cfg.Sync.ApplySpells,
		DryRun:      cfg.Sync.DryRun,
		Debounce:    time.Duration(cfg.Watch.DebounceMillis) * time.Millisecond,
		OnCycle:     func(c lolautobuild.WatchCycle) { a.observeWatchCycle(watcherID, c) },
	})

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherID != watcherID {
		return
	}

	a.cancelWatcher = nil
	a.watcherRunning = false

	if err != nil && ctx.Err() == nil {
		a.lastErrorMessage = messageFromErr(err)
	}
}

func (a *App) reserveWatcherStart() (cfg config.Config, watcherID int, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherStarting || a.watcherRunning {
		return config.Config{}, 0, false
	}

	a.watcherID++
	a.watcherStarting = true
	a.lastErrorMessage = messageFromErr(nil)

	return a.cfg, a.watcherID, true
}

func (a *App) startReservedWatcher(watcherID int) (watcherCtx context.Context, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.watcherID != watcherID || !a.watcherStarting {
		return nil, false
	}

	watcherCtx, a.cancelWatcher = context.WithCancel(context.Background())
	a.watcherStarting = false
	a.watcherRunning = true

	return watcherCtx, true
}

func (a *App) observeWatchCycle(watchID int, c lolautobuild.WatchCycle) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.watcherRunning || watchID != a.watcherID {
		return
	}

	if c.Err != nil {
		a.lastErrorMessage = messageFromErr(c.Err)
		return
	}

	if c.Result != nil {
		a.lastSync = cloneSyncResult(c.Result)
		a.lastSyncAt = time.Now().UTC()
	}

	a.lastErrorMessage = messageFromErr(nil)
}

func (a *App) StopWatcher(ctx context.Context) State {
	a.mu.Lock()
	cancel := a.cancelWatcher
	a.cancelWatcher = nil
	a.watcherStarting = false
	a.watcherRunning = false
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

	if err != nil {
		a.lastErrorMessage = messageFromErr(err)
	}
}

func (a *App) RunSync(ctx context.Context) (State, string) {
	cfg, alreadySyncing := a.beginSync()
	if alreadySyncing {
		return State{}, "Another sync is already running"
	}

	svc, err := a.serviceFactory(cfg)
	if err != nil {
		a.finishSync(nil, err)
		return State{}, messageFromErr(err)
	}

	result, err := svc.Sync(ctx, lolautobuild.SyncRequest{
		Patch:       cfg.Sync.Patch,
		ApplyItems:  cfg.Sync.ApplyItems,
		ApplyRunes:  cfg.Sync.ApplyRunes,
		ApplySpells: cfg.Sync.ApplySpells,
		DryRun:      cfg.Sync.DryRun,
	})
	a.finishSync(&result, err)
	if err != nil {
		return State{}, messageFromErr(err)
	}

	return a.State(ctx), messageFromErr(nil)
}

func (a *App) beginSync() (configSnapshot config.Config, alreadySyncing bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.syncRunning {
		return config.Config{}, true
	}

	a.syncRunning = true
	a.lastErrorMessage = messageFromErr(nil)
	return a.cfg, false
}

func (a *App) finishSync(res *lolautobuild.SyncResult, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.syncRunning = false

	if err != nil {
		a.lastErrorMessage = messageFromErr(err)
		return
	}

	a.lastSync = cloneSyncResult(res)
	a.lastSyncAt = time.Now().UTC()
	a.lastErrorMessage = messageFromErr(nil)
}

func cloneSyncResult(res *lolautobuild.SyncResult) *lolautobuild.SyncResult {
	if res == nil {
		return nil
	}

	out := *res
	out.Warnings = append([]string{}, res.Warnings...)
	return &out
}

func (a *App) CheckUpdates(ctx context.Context) (State, string) {
	if !a.beginUpdateCheck() {
		return a.State(ctx), ""
	}

	result, err := a.updateChecker.Check(ctx)
	a.finishUpdateCheck(result, err)

	return a.State(ctx), ""
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

func (a *App) finishUpdateCheck(result update.Result, err error) {
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
	case errors.Is(err, update.ErrUnavailable):
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
