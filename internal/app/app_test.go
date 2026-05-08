package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild"
)

const testTimeout = 2 * time.Second

type testAppOptions struct {
	cfg            RuntimeConfig
	serviceFactory ServiceFactory
	lcuStatus      LCUStatusProvider
	coachlessAuth  CoachlessAuthSession
	updateChecker  UpdateChecker
	configStore    ConfigStore
	messageFromErr MessageMapper
}

func newTestApp(t *testing.T, opts testAppOptions) *App {
	t.Helper()

	if opts.cfg == (RuntimeConfig{}) {
		opts.cfg = testConfig()
	}
	if opts.serviceFactory == nil {
		opts.serviceFactory = func(RuntimeConfig) (autobuild.Service, error) {
			return newStubService(), nil
		}
	}
	if opts.lcuStatus == nil {
		opts.lcuStatus = func(context.Context, RuntimeConfig) LCUStatus {
			return LCUStatus{State: LCUConnectionStateOff, Message: "LCU is off"}
		}
	}
	if opts.updateChecker == nil {
		opts.updateChecker = &stubUpdateChecker{currentVersion: "0.1.0"}
	}
	if opts.configStore == nil {
		opts.configStore = &recordingConfigStore{}
	}

	app, err := New(Options{
		ServiceFactory:   opts.serviceFactory,
		LCUStatus:        opts.lcuStatus,
		CoachlessAuth:    opts.coachlessAuth,
		UpdateChecker:    opts.updateChecker,
		ConfigStore:      opts.configStore,
		RuntimeConfig:    opts.cfg,
		MessageFromError: opts.messageFromErr,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return app
}

type stubUpdateChecker struct {
	mu             sync.Mutex
	currentVersion string
	checkFn        func(context.Context) (UpdateCheckResult, error)
	calls          int
}

func (s *stubUpdateChecker) CurrentVersion() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.currentVersion
}

func (s *stubUpdateChecker) Check(ctx context.Context) (UpdateCheckResult, error) {
	s.mu.Lock()
	s.calls++
	fn := s.checkFn
	currentVersion := s.currentVersion
	s.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}

	return UpdateCheckResult{CurrentVersion: currentVersion, LatestVersion: currentVersion}, nil
}

func (s *stubUpdateChecker) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

type recordingConfigStore struct {
	mu       sync.Mutex
	saveErr  error
	savedCfg []RuntimeConfig
}

func (s *recordingConfigStore) Save(newCfg RuntimeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.savedCfg = append(s.savedCfg, newCfg)
	return s.saveErr
}

func (s *recordingConfigStore) saveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.savedCfg)
}

func (s *recordingConfigStore) lastSaved() RuntimeConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.savedCfg) == 0 {
		return RuntimeConfig{}
	}

	return s.savedCfg[len(s.savedCfg)-1]
}

type syncCall struct {
	ctx context.Context
	req autobuild.SyncRequest
}

type watchCall struct {
	ctx context.Context
	req autobuild.WatchRequest
}

type stubService struct {
	mu sync.Mutex

	syncFn  func(context.Context, autobuild.SyncRequest) (autobuild.SyncResult, error)
	watchFn func(context.Context, autobuild.WatchRequest) error

	syncCalls  []syncCall
	watchCalls []watchCall

	syncCalled  chan syncCall
	watchCalled chan watchCall
}

type stubCoachlessAuth struct {
	mu sync.Mutex

	status CoachlessAuthState
	err    error

	loginCalls  int
	logoutCalls int
}

func (s *stubCoachlessAuth) Status(context.Context) CoachlessAuthState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneCoachlessAuthState(s.status)
}

func (s *stubCoachlessAuth) Login(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loginCalls++
	return s.err
}

func (s *stubCoachlessAuth) Logout(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logoutCalls++
	return s.err
}

func newStubService() *stubService {
	return &stubService{
		syncCalled:  make(chan syncCall, 8),
		watchCalled: make(chan watchCall, 8),
	}
}

func (s *stubService) Sync(ctx context.Context, req autobuild.SyncRequest) (autobuild.SyncResult, error) {
	call := syncCall{ctx: ctx, req: req}

	s.mu.Lock()
	s.syncCalls = append(s.syncCalls, call)
	fn := s.syncFn
	ch := s.syncCalled
	s.mu.Unlock()

	if ch != nil {
		select {
		case ch <- call:
		default:
		}
	}

	if fn != nil {
		return fn(ctx, req)
	}

	return autobuild.SyncResult{}, nil
}

func (s *stubService) Watch(ctx context.Context, req autobuild.WatchRequest) error {
	call := watchCall{ctx: ctx, req: req}

	s.mu.Lock()
	s.watchCalls = append(s.watchCalls, call)
	fn := s.watchFn
	ch := s.watchCalled
	s.mu.Unlock()

	if ch != nil {
		select {
		case ch <- call:
		default:
		}
	}

	if fn != nil {
		return fn(ctx, req)
	}

	return nil
}

func (s *stubService) EnsureCoachlessAuth(ctx context.Context) error {
	return nil
}

func (s *stubService) syncCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.syncCalls)
}

func (s *stubService) watchCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.watchCalls)
}

func TestStateReturnsSnapshotAndCopy(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "14.8"
	cfg.Settings.ApplyRunes = false
	cfg.Settings.DryRun = false
	cfg.Settings.LCUEnabled = true

	wantStatus := LCUStatus{
		State:  LCUConnectionStateConnected,
		Source: "status-check",
	}

	statusCalls := 0
	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		lcuStatus: func(_ context.Context, got RuntimeConfig) LCUStatus {
			statusCalls++
			if got != cfg {
				t.Fatalf("status checker received config %+v, want %+v", got, cfg)
			}
			return wantStatus
		},
	})

	lastSyncAt := time.Date(2026, time.April, 25, 12, 30, 0, 0, time.UTC)
	wantLastSync := &autobuild.SyncResult{
		DetectedChampionID:   238,
		DetectedChampionName: "Zed",
		DetectedPosition:     "mid",
		DetectedQueueID:      420,
		ItemSetApplied:       true,
		RunePageApplied:      true,
		SpellsApplied:        false,
		Warnings:             []string{"low sample"},
	}

	app.syncRunning = true
	app.watcherRunning = true
	app.lastErrorMessage = "previous error"
	app.lastErrorCode = "test.error"
	app.lastSync = syncSummaryFromResult(*wantLastSync)
	app.lastSyncAt = lastSyncAt
	app.updateState = UpdateState{
		Status:         UpdateStatusAvailable,
		CurrentVersion: "0.1.0",
		LatestVersion:  "v0.2.0",
		DownloadURL:    "https://example.test/latest",
		CheckedAt:      &lastSyncAt,
		Message:        "Download v0.2.0.",
	}

	state := app.State(context.Background())
	if statusCalls != 1 {
		t.Fatalf("status checker calls = %d, want 1", statusCalls)
	}

	if state.Settings != cfg.Settings {
		t.Fatalf("state.Settings = %+v, want %+v", state.Settings, cfg.Settings)
	}
	if state.LCU != wantStatus {
		t.Fatalf("state.LCU = %+v, want %+v", state.LCU, wantStatus)
	}
	if !state.Watcher.Running {
		t.Fatal("expected watcher to be running")
	}
	if state.Update.Status != UpdateStatusAvailable {
		t.Fatalf("state.Update.Status = %q, want %q", state.Update.Status, UpdateStatusAvailable)
	}
	if state.Update.CheckedAt == nil || !state.Update.CheckedAt.Equal(lastSyncAt) {
		t.Fatalf("state.Update.CheckedAt = %v, want %v", state.Update.CheckedAt, lastSyncAt)
	}
	if state.Update.CheckedAt == app.updateState.CheckedAt {
		t.Fatal("state.Update.CheckedAt should be a copy")
	}
	if !state.SyncRunning {
		t.Fatal("expected sync to be running")
	}
	if state.LastError != "previous error" {
		t.Fatalf("state.LastError = %q, want %q", state.LastError, "previous error")
	}
	if state.LastErrorCode != "test.error" {
		t.Fatalf("state.LastErrorCode = %q, want %q", state.LastErrorCode, "test.error")
	}
	assertSyncResultEqual(t, state.LastSync, wantLastSync)
	if state.LastSync == app.lastSync {
		t.Fatal("state.LastSync should be a copy")
	}
	if state.LastSyncAt == nil || !state.LastSyncAt.Equal(lastSyncAt) {
		t.Fatalf("state.LastSyncAt = %v, want %v", state.LastSyncAt, lastSyncAt)
	}

	state.LastSync.Warnings[0].Fallback = "mutated"
	state.LastSync.Warnings = append(state.LastSync.Warnings, MessageDescriptor{Fallback: "extra"})
	*state.LastSyncAt = state.LastSyncAt.Add(5 * time.Minute)
	*state.Update.CheckedAt = state.Update.CheckedAt.Add(5 * time.Minute)

	next := app.State(context.Background())
	assertSyncResultEqual(t, next.LastSync, wantLastSync)
	if next.LastSyncAt == nil || !next.LastSyncAt.Equal(lastSyncAt) {
		t.Fatalf("next.LastSyncAt = %v, want %v", next.LastSyncAt, lastSyncAt)
	}
	if next.Update.CheckedAt == nil || !next.Update.CheckedAt.Equal(lastSyncAt) {
		t.Fatalf("next.Update.CheckedAt = %v, want %v", next.Update.CheckedAt, lastSyncAt)
	}
}

func TestStateCopiesCoachlessAuthState(t *testing.T) {
	expiresAt := time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC)
	authSession := &stubCoachlessAuth{status: CoachlessAuthState{
		Status:    CoachlessAuthStatusStored,
		Plan:      CoachlessAuthPlanPremium,
		ExpiresAt: &expiresAt,
	}}
	app := newTestApp(t, testAppOptions{coachlessAuth: authSession})

	state := app.State(context.Background())
	if state.CoachlessAuth.Status != CoachlessAuthStatusStored || state.CoachlessAuth.Plan != CoachlessAuthPlanPremium {
		t.Fatalf("CoachlessAuth = %+v", state.CoachlessAuth)
	}
	if state.CoachlessAuth.ExpiresAt == nil || !state.CoachlessAuth.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("CoachlessAuth.ExpiresAt = %v, want %v", state.CoachlessAuth.ExpiresAt, expiresAt)
	}

	*state.CoachlessAuth.ExpiresAt = state.CoachlessAuth.ExpiresAt.Add(time.Hour)
	next := app.State(context.Background())
	if next.CoachlessAuth.ExpiresAt == nil || !next.CoachlessAuth.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("next CoachlessAuth.ExpiresAt = %v, want %v", next.CoachlessAuth.ExpiresAt, expiresAt)
	}
}

func TestStateBoundsLCUStatusRefresh(t *testing.T) {
	app := newTestApp(t, testAppOptions{
		lcuStatus: func(ctx context.Context, _ RuntimeConfig) LCUStatus {
			<-ctx.Done()
			return LCUStatus{State: LCUConnectionStateNotConnected, Message: ctx.Err().Error()}
		},
	})

	start := time.Now()
	state := app.State(context.Background())
	elapsed := time.Since(start)

	if elapsed > lcuStateRefreshTimeout+500*time.Millisecond {
		t.Fatalf("State() took %v, want bounded by LCU state refresh timeout", elapsed)
	}
	if state.LCU.State != LCUConnectionStateNotConnected {
		t.Fatalf("LCU state = %q, want not_connected", state.LCU.State)
	}
	if state.LCU.Message != context.DeadlineExceeded.Error() {
		t.Fatalf("LCU message = %q, want deadline exceeded", state.LCU.Message)
	}
}

func TestSyncSummaryMapsKnownWarningsToMessageDescriptors(t *testing.T) {
	summary := syncSummaryFromResult(autobuild.SyncResult{
		Warnings: []string{
			autobuild.RunePageLimitReachedWarning,
			"low sample",
		},
	})

	wantWarnings := []MessageDescriptor{
		{Key: MessageCodeSyncRunePageLimitReached, Fallback: autobuild.RunePageLimitReachedWarning},
		{Fallback: "low sample"},
	}
	if !reflect.DeepEqual(summary.Warnings, wantWarnings) {
		t.Fatalf("warnings = %+v, want %+v", summary.Warnings, wantWarnings)
	}
}

func TestCoachlessAuthActionsUseSessionWithoutSavingConfig(t *testing.T) {
	store := &recordingConfigStore{}
	authSession := &stubCoachlessAuth{
		status: CoachlessAuthState{Status: CoachlessAuthStatusStored, Plan: CoachlessAuthPlanUnknown},
	}
	app := newTestApp(t, testAppOptions{configStore: store, coachlessAuth: authSession})
	app.lastErrorMessage = "old error"

	state, message := app.LoginCoachlessAuth(context.Background())
	if !message.Empty() {
		t.Fatalf("LoginCoachlessAuth() message = %+v, want empty", message)
	}
	if state.CoachlessAuth.Status != CoachlessAuthStatusStored {
		t.Fatalf("state.CoachlessAuth = %+v", state.CoachlessAuth)
	}

	state, message = app.LogoutCoachlessAuth(context.Background())
	if !message.Empty() {
		t.Fatalf("LogoutCoachlessAuth() message = %+v, want empty", message)
	}

	if authSession.loginCalls != 1 || authSession.logoutCalls != 1 {
		t.Fatalf("auth calls = login:%d logout:%d", authSession.loginCalls, authSession.logoutCalls)
	}
	if store.saveCount() != 0 {
		t.Fatalf("config saves = %d, want 0", store.saveCount())
	}
	if state.LastError != "" {
		t.Fatalf("LastError = %q, want empty", state.LastError)
	}
}

func TestCoachlessAuthActionErrorRecordsLastError(t *testing.T) {
	authSession := &stubCoachlessAuth{err: errors.New("auth failed")}
	app := newTestApp(t, testAppOptions{coachlessAuth: authSession})

	state, message := app.LoginCoachlessAuth(context.Background())
	if message.Text != "auth failed" {
		t.Fatalf("LoginCoachlessAuth() message = %+v", message)
	}
	if state != (ViewState{}) {
		t.Fatalf("LoginCoachlessAuth() state = %+v, want zero", state)
	}

	current := app.State(context.Background())
	if current.LastError != "auth failed" {
		t.Fatalf("LastError = %q, want auth failed", current.LastError)
	}
}

func TestSaveSettingsPersistsTrimmedSettings(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.LCUEnabled = false

	store := &recordingConfigStore{}
	factoryCalls := 0
	app := newTestApp(t, testAppOptions{
		cfg:         cfg,
		configStore: store,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			factoryCalls++
			return newStubService(), nil
		},
		lcuStatus: func(_ context.Context, got RuntimeConfig) LCUStatus {
			if got.Settings.LCUEnabled {
				return LCUStatus{State: LCUConnectionStateConnected, Source: "settings"}
			}
			return LCUStatus{State: LCUConnectionStateOff, Message: "LCU is off"}
		},
	})

	app.lastErrorMessage = "stale error"

	settings := Settings{
		Patch:              " 14.9 ",
		PatchAdditionsMode: " " + autobuild.PatchAdditionsModeManual + " ",
		PatchAdditions:     autobuild.PatchAdditionsMax,
		LeagueTierPreset:   " " + autobuild.LeagueTierPresetMasterPlus + " ",
		ApplyItems:         false,
		ApplyRunes:         true,
		ApplySpells:        false,
		KeepFlash:          false,
		DryRun:             false,
		LCUEnabled:         true,
	}

	state, message := app.SaveSettings(context.Background(), settings)
	if !message.Empty() {
		t.Fatalf("SaveSettings() message = %q, want empty", message.Text)
	}
	if factoryCalls != 0 {
		t.Fatalf("serviceFactory calls = %d, want 0", factoryCalls)
	}
	if store.saveCount() != 1 {
		t.Fatalf("Save() calls = %d, want 1", store.saveCount())
	}

	wantCfg := cfg
	wantCfg.Settings = normalizeSettings(settings)

	if got := store.lastSaved(); got != wantCfg {
		t.Fatalf("saved config = %+v, want %+v", got, wantCfg)
	}
	if state.Settings != wantCfg.Settings {
		t.Fatalf("state.Settings = %+v, want %+v", state.Settings, wantCfg.Settings)
	}
	if state.LastError != "" {
		t.Fatalf("state.LastError = %q, want empty", state.LastError)
	}
	if state.LCU.State != LCUConnectionStateConnected {
		t.Fatalf("state.LCU.State = %q, want %q", state.LCU.State, LCUConnectionStateConnected)
	}
}

func TestSaveSettingsReturnsErrorWithoutMutatingConfig(t *testing.T) {
	cfg := testConfig()
	store := &recordingConfigStore{saveErr: errors.New("save failed")}

	factoryCalls := 0
	cancelCalled := false
	app := newTestApp(t, testAppOptions{
		cfg:         cfg,
		configStore: store,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			factoryCalls++
			return newStubService(), nil
		},
	})

	app.watcherRunning = true
	app.cancelWatcher = func() { cancelCalled = true }
	app.lastErrorMessage = "previous error"

	state, message := app.SaveSettings(context.Background(), Settings{
		Patch:       " 15.1 ",
		ApplyItems:  false,
		ApplyRunes:  false,
		ApplySpells: false,
		KeepFlash:   false,
		DryRun:      false,
		LCUEnabled:  true,
	})

	if message.Text != "save failed" {
		t.Fatalf("SaveSettings() message = %q, want %q", message.Text, "save failed")
	}
	if state != (ViewState{}) {
		t.Fatalf("SaveSettings() state = %+v, want zero value", state)
	}
	if factoryCalls != 0 {
		t.Fatalf("serviceFactory calls = %d, want 0", factoryCalls)
	}
	if cancelCalled {
		t.Fatal("watcher cancel should not be called on save failure")
	}

	current := app.State(context.Background())
	if current.Settings != cfg.Settings {
		t.Fatalf("current.Settings = %+v, want %+v", current.Settings, cfg.Settings)
	}
	if !current.Watcher.Running {
		t.Fatal("expected watcher to remain running after save failure")
	}
	if current.LastError != "previous error" {
		t.Fatalf("current.LastError = %q, want %q", current.LastError, "previous error")
	}
}

func TestSaveSettingsMarksRunningWatcherConfigStaleWithoutRestart(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "14.10"

	store := &recordingConfigStore{}
	firstSvc := newStubService()
	firstStopped := make(chan struct{})

	firstSvc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(firstStopped)
		return nil
	}

	var (
		factoryMu   sync.Mutex
		factoryCfgs []RuntimeConfig
	)
	app := newTestApp(t, testAppOptions{
		cfg:         cfg,
		configStore: store,
		serviceFactory: func(got RuntimeConfig) (autobuild.Service, error) {
			factoryMu.Lock()
			defer factoryMu.Unlock()
			factoryCfgs = append(factoryCfgs, got)
			return firstSvc, nil
		},
	})

	startState, message := app.StartWatcher(context.Background())
	if !message.Empty() {
		t.Fatalf("StartWatcher() message = %q, want empty", message.Text)
	}
	if !startState.Watcher.Running {
		t.Fatal("expected watcher to be running after first start")
	}
	if startState.Watcher.ConfigStale {
		t.Fatal("expected watcher config to start fresh")
	}

	firstCall := waitForWatchCall(t, firstSvc)
	assertWatchRequestMatchesConfig(t, firstCall.req, cfg)

	newSettings := Settings{
		Patch:              " 15.2 ",
		PatchAdditionsMode: autobuild.PatchAdditionsModeManual,
		PatchAdditions:     1,
		LeagueTierPreset:   autobuild.LeagueTierPresetPlatinumPlus,
		ApplyItems:         false,
		ApplyRunes:         true,
		ApplySpells:        false,
		KeepFlash:          false,
		DryRun:             false,
		LCUEnabled:         true,
	}

	wantCfg := cfg
	wantCfg.Settings = normalizeSettings(newSettings)

	state, message := app.SaveSettings(context.Background(), newSettings)
	if !message.Empty() {
		t.Fatalf("SaveSettings() message = %q, want empty", message.Text)
	}

	if store.saveCount() != 1 {
		t.Fatalf("Save() calls = %d, want 1", store.saveCount())
	}
	if got := store.lastSaved(); got != wantCfg {
		t.Fatalf("saved config = %+v, want %+v", got, wantCfg)
	}

	factoryMu.Lock()
	gotFactoryCfgs := append([]RuntimeConfig(nil), factoryCfgs...)
	factoryMu.Unlock()

	if len(gotFactoryCfgs) != 1 {
		t.Fatalf("factory configs recorded = %d, want 1", len(gotFactoryCfgs))
	}
	if gotFactoryCfgs[0] != cfg {
		t.Fatalf("first factory config = %+v, want %+v", gotFactoryCfgs[0], cfg)
	}

	select {
	case <-firstStopped:
		t.Fatal("watcher should not stop during settings save")
	default:
	}

	if !state.Watcher.Running {
		t.Fatal("expected watcher to remain running after save")
	}
	if !state.Watcher.ConfigStale {
		t.Fatal("expected watcher config to be stale after save")
	}
	if state.Settings != wantCfg.Settings {
		t.Fatalf("state.Settings = %+v, want %+v", state.Settings, wantCfg.Settings)
	}

	stopped := app.StopWatcher(context.Background())
	if stopped.Watcher.Running {
		t.Fatal("expected watcher to stop")
	}
	if stopped.Watcher.ConfigStale {
		t.Fatal("expected stopped watcher config to be fresh")
	}
	waitForSignal(t, firstStopped, "first watcher stop")
}

func TestSaveSettingsKeepsRunningWatcherConfigFreshForEquivalentConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "14.10"

	store := &recordingConfigStore{}
	svc := newStubService()
	watchStopped := make(chan struct{})
	svc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(watchStopped)
		return nil
	}

	app := newTestApp(t, testAppOptions{
		cfg:         cfg,
		configStore: store,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			return svc, nil
		},
	})

	if state, message := app.StartWatcher(context.Background()); !message.Empty() || !state.Watcher.Running {
		t.Fatalf("StartWatcher() state = %+v, message = %q", state, message.Text)
	}
	waitForWatchCall(t, svc)

	state, message := app.SaveSettings(context.Background(), cfg.Settings)
	if !message.Empty() {
		t.Fatalf("SaveSettings() message = %q, want empty", message.Text)
	}
	if state.Watcher.ConfigStale {
		t.Fatal("expected equivalent settings to keep watcher config fresh")
	}
	if !state.Watcher.Running {
		t.Fatal("expected watcher to remain running")
	}

	_ = app.StopWatcher(context.Background())
	waitForSignal(t, watchStopped, "watcher stop")
}

func TestStartWatcherUsesLatestSavedConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "14.10"

	store := &recordingConfigStore{}
	svc := newStubService()
	watchStopped := make(chan struct{})
	svc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(watchStopped)
		return nil
	}

	var gotFactoryCfg RuntimeConfig
	app := newTestApp(t, testAppOptions{
		cfg:         cfg,
		configStore: store,
		serviceFactory: func(got RuntimeConfig) (autobuild.Service, error) {
			gotFactoryCfg = got
			return svc, nil
		},
	})

	settings := cfg.Settings
	settings.Patch = "15.2"
	settings.ApplyItems = false
	wantCfg := cfg
	wantCfg.Settings = normalizeSettings(settings)

	if _, message := app.SaveSettings(context.Background(), settings); !message.Empty() {
		t.Fatalf("SaveSettings() message = %q, want empty", message.Text)
	}

	state, message := app.StartWatcher(context.Background())
	if !message.Empty() {
		t.Fatalf("StartWatcher() message = %q, want empty", message.Text)
	}
	if state.Watcher.ConfigStale {
		t.Fatal("expected started watcher config to be fresh")
	}
	if gotFactoryCfg != wantCfg {
		t.Fatalf("factory config = %+v, want %+v", gotFactoryCfg, wantCfg)
	}

	call := waitForWatchCall(t, svc)
	assertWatchRequestMatchesConfig(t, call.req, wantCfg)

	_ = app.StopWatcher(context.Background())
	waitForSignal(t, watchStopped, "watcher stop")
}

func TestStartWatcherLifecycle(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "15.3"
	cfg.Settings.ApplyItems = true
	cfg.Settings.ApplyRunes = false
	cfg.Settings.ApplySpells = true
	cfg.Settings.KeepFlash = true
	cfg.Settings.DryRun = false
	cfg.WatchDebounce = 321 * time.Millisecond

	wantStatus := LCUStatus{
		State:  LCUConnectionStateConnected,
		Source: "lockfile",
	}

	svc := newStubService()
	watchStopped := make(chan struct{})
	svc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(watchStopped)
		return nil
	}

	var factoryCfg RuntimeConfig
	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(got RuntimeConfig) (autobuild.Service, error) {
			factoryCfg = got
			return svc, nil
		},
		lcuStatus: func(context.Context, RuntimeConfig) LCUStatus {
			return wantStatus
		},
	})

	state, message := app.StartWatcher(context.Background())
	if !message.Empty() {
		t.Fatalf("StartWatcher() message = %q, want empty", message.Text)
	}
	if !state.Watcher.Running {
		t.Fatal("expected watcher to be running")
	}
	if state.LCU != wantStatus {
		t.Fatalf("state.LCU = %+v, want %+v", state.LCU, wantStatus)
	}
	if factoryCfg != cfg {
		t.Fatalf("factory config = %+v, want %+v", factoryCfg, cfg)
	}

	call := waitForWatchCall(t, svc)
	assertWatchRequestMatchesConfig(t, call.req, cfg)

	beforeSuccess := time.Now().UTC()
	wantResult := &autobuild.SyncResult{
		DetectedChampionID:   240,
		DetectedChampionName: "Kled",
		DetectedPosition:     "mid",
		DetectedQueueID:      420,
		ItemSetApplied:       true,
		RunePageApplied:      true,
		SpellsApplied:        true,
		Warnings:             []string{"manual review"},
	}
	call.req.OnCycle(autobuild.WatchCycle{Result: wantResult})
	afterSuccess := time.Now().UTC()

	current := app.State(context.Background())
	assertSyncResultEqual(t, current.LastSync, wantResult)
	assertTimeBetween(t, current.LastSyncAt, beforeSuccess, afterSuccess)
	if current.LastError != "" {
		t.Fatalf("current.LastError = %q, want empty", current.LastError)
	}

	beforeNotice := time.Now().UTC()
	call.req.OnNotice(autobuild.WatchNotice{
		Kind:         autobuild.WatchNoticeSnapshotFinalization,
		Message:      "snapshot finalized",
		URI:          "/lol-champ-select/v1/session",
		Phase:        "FINALIZATION",
		ConnectionID: 2,
	})
	afterNotice := time.Now().UTC()

	current = app.State(context.Background())
	if current.Watcher.LastNotice == nil {
		t.Fatal("expected watcher notice in state")
	}
	if current.Watcher.LastNotice.Kind != string(autobuild.WatchNoticeSnapshotFinalization) {
		t.Fatalf("notice kind = %q, want %q", current.Watcher.LastNotice.Kind, autobuild.WatchNoticeSnapshotFinalization)
	}
	if current.Watcher.LastNotice.Phase != "FINALIZATION" || current.Watcher.LastNotice.ConnectionID != 2 {
		t.Fatalf("unexpected watcher notice: %+v", current.Watcher.LastNotice)
	}
	assertTimeBetween(t, &current.Watcher.LastNotice.At, beforeNotice, afterNotice)
	assertSyncResultEqual(t, current.LastSync, wantResult)

	call.req.OnCycle(autobuild.WatchCycle{Err: errors.New("watch cycle failed")})
	current = app.State(context.Background())
	if current.LastError != "watch cycle failed" {
		t.Fatalf("current.LastError = %q, want %q", current.LastError, "watch cycle failed")
	}
	assertSyncResultEqual(t, current.LastSync, wantResult)

	stopped := app.StopWatcher(context.Background())
	if stopped.Watcher.Running {
		t.Fatal("expected watcher to stop")
	}
	waitForSignal(t, watchStopped, "watch stop")
}

func TestStartWatcherRejectsSecondStart(t *testing.T) {
	cfg := testConfig()

	svc := newStubService()
	watchStopped := make(chan struct{})
	svc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(watchStopped)
		return nil
	}

	factoryCalls := 0
	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			factoryCalls++
			return svc, nil
		},
	})

	if _, message := app.StartWatcher(context.Background()); !message.Empty() {
		t.Fatalf("StartWatcher() message = %q, want empty", message.Text)
	}
	_ = waitForWatchCall(t, svc)

	state, message := app.StartWatcher(context.Background())
	if message.Text != "Watcher pre-start failed." {
		t.Fatalf("second StartWatcher() message = %q, want %q", message.Text, "Watcher start failed.")
	}
	if message.Code != MessageCodeWatcherPreStartFailed {
		t.Fatalf("second StartWatcher() code = %q, want %q", message.Code, MessageCodeWatcherPreStartFailed)
	}
	if !state.Watcher.Running {
		t.Fatal("expected original watcher to keep running")
	}
	if factoryCalls != 1 {
		t.Fatalf("serviceFactory calls = %d, want 1", factoryCalls)
	}
	if svc.watchCallCount() != 1 {
		t.Fatalf("watch calls = %d, want 1", svc.watchCallCount())
	}

	app.StopWatcher(context.Background())
	waitForSignal(t, watchStopped, "watch stop")
}

func TestStartWatcherReleasesReservationOnFactoryError(t *testing.T) {
	cfg := testConfig()
	factoryErr := errors.New("factory failed")

	svc := newStubService()
	watchStopped := make(chan struct{})
	svc.watchFn = func(ctx context.Context, req autobuild.WatchRequest) error {
		<-ctx.Done()
		close(watchStopped)
		return nil
	}

	factoryCalls := 0
	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return nil, factoryErr
			}
			return svc, nil
		},
	})

	state, message := app.StartWatcher(context.Background())
	if message.Text != "factory failed" {
		t.Fatalf("StartWatcher() message = %q, want %q", message.Text, "factory failed")
	}
	if state != (ViewState{}) {
		t.Fatalf("StartWatcher() state = %+v, want zero value", state)
	}

	current := app.State(context.Background())
	if current.Watcher.Running {
		t.Fatal("expected watcher reservation to be released")
	}
	if current.LastError != "factory failed" {
		t.Fatalf("current.LastError = %q, want %q", current.LastError, "factory failed")
	}

	started, message := app.StartWatcher(context.Background())
	if !message.Empty() {
		t.Fatalf("second StartWatcher() message = %q, want empty", message.Text)
	}
	if !started.Watcher.Running {
		t.Fatal("expected watcher to start after factory error")
	}
	_ = waitForWatchCall(t, svc)

	app.StopWatcher(context.Background())
	waitForSignal(t, watchStopped, "watch stop")
}

func TestRunSyncSuccess(t *testing.T) {
	cfg := testConfig()
	cfg.Settings.Patch = "15.4"
	cfg.Settings.ApplyItems = true
	cfg.Settings.ApplyRunes = false
	cfg.Settings.ApplySpells = false
	cfg.Settings.DryRun = false

	wantResult := autobuild.SyncResult{
		DetectedChampionID:   84,
		DetectedChampionName: "Akali",
		DetectedPosition:     "support",
		DetectedQueueID:      420,
		ItemSetApplied:       true,
		RunePageApplied:      false,
		SpellsApplied:        false,
		Warnings:             []string{"partial sync"},
	}

	svc := newStubService()
	svc.syncFn = func(context.Context, autobuild.SyncRequest) (autobuild.SyncResult, error) {
		return wantResult, nil
	}

	var factoryCfg RuntimeConfig
	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(got RuntimeConfig) (autobuild.Service, error) {
			factoryCfg = got
			return svc, nil
		},
	})
	app.lastErrorMessage = "stale error"

	before := time.Now().UTC()
	state, message := app.RunSync(context.Background())
	after := time.Now().UTC()

	if !message.Empty() {
		t.Fatalf("RunSync() message = %q, want empty", message.Text)
	}
	if factoryCfg != cfg {
		t.Fatalf("factory config = %+v, want %+v", factoryCfg, cfg)
	}

	call := waitForSyncCall(t, svc)
	assertSyncRequestMatchesConfig(t, call.req, cfg)
	if state.SyncRunning {
		t.Fatal("expected sync to be finished")
	}
	if state.LastError != "" {
		t.Fatalf("state.LastError = %q, want empty", state.LastError)
	}
	assertSyncResultEqual(t, state.LastSync, &wantResult)
	assertTimeBetween(t, state.LastSyncAt, before, after)
}

func TestRunSyncFailureCases(t *testing.T) {
	tests := []struct {
		name          string
		factoryErr    error
		syncErr       error
		wantMessage   string
		wantSyncCalls int
	}{
		{
			name:        "factory error",
			factoryErr:  errors.New("factory failed"),
			wantMessage: "factory failed",
		},
		{
			name:          "sync error",
			syncErr:       errors.New("sync failed"),
			wantMessage:   "sync failed",
			wantSyncCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			oldSyncAt := time.Date(2026, time.April, 25, 8, 0, 0, 0, time.UTC)
			oldSync := &autobuild.SyncResult{
				DetectedChampionID: 55,
				Warnings:           []string{"keep me"},
			}

			svc := newStubService()
			svc.syncFn = func(context.Context, autobuild.SyncRequest) (autobuild.SyncResult, error) {
				return autobuild.SyncResult{DetectedChampionID: 999}, tt.syncErr
			}

			app := newTestApp(t, testAppOptions{
				cfg: cfg,
				serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
					if tt.factoryErr != nil {
						return nil, tt.factoryErr
					}
					return svc, nil
				},
			})
			app.lastSync = syncSummaryFromResult(*oldSync)
			app.lastSyncAt = oldSyncAt

			state, message := app.RunSync(context.Background())
			if message.Text != tt.wantMessage {
				t.Fatalf("RunSync() message = %q, want %q", message.Text, tt.wantMessage)
			}
			if state != (ViewState{}) {
				t.Fatalf("RunSync() state = %+v, want zero value", state)
			}
			if svc.syncCallCount() != tt.wantSyncCalls {
				t.Fatalf("sync calls = %d, want %d", svc.syncCallCount(), tt.wantSyncCalls)
			}

			current := app.State(context.Background())
			if current.SyncRunning {
				t.Fatal("expected syncRunning to be released after failure")
			}
			if current.LastError != tt.wantMessage {
				t.Fatalf("current.LastError = %q, want %q", current.LastError, tt.wantMessage)
			}
			assertSyncResultEqual(t, current.LastSync, oldSync)
			if current.LastSyncAt == nil || !current.LastSyncAt.Equal(oldSyncAt) {
				t.Fatalf("current.LastSyncAt = %v, want %v", current.LastSyncAt, oldSyncAt)
			}
		})
	}
}

func TestRunSyncRejectsConcurrentCalls(t *testing.T) {
	cfg := testConfig()

	blockSync := make(chan struct{})
	startedSync := make(chan struct{})

	svc := newStubService()
	svc.syncFn = func(context.Context, autobuild.SyncRequest) (autobuild.SyncResult, error) {
		close(startedSync)
		<-blockSync
		return autobuild.SyncResult{DetectedChampionID: 99}, nil
	}

	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			return svc, nil
		},
	})

	type runSyncResult struct {
		state   ViewState
		message UserMessage
	}

	firstDone := make(chan runSyncResult, 1)
	go func() {
		state, message := app.RunSync(context.Background())
		firstDone <- runSyncResult{state: state, message: message}
	}()

	waitForSignal(t, startedSync, "first sync start")

	state, message := app.RunSync(context.Background())
	if message.Text != "Another sync is already running" {
		t.Fatalf("second RunSync() message = %q, want %q", message.Text, "Another sync is already running")
	}
	if message.Code != MessageCodeSyncAlreadyRunning {
		t.Fatalf("second RunSync() code = %q, want %q", message.Code, MessageCodeSyncAlreadyRunning)
	}
	if state != (ViewState{}) {
		t.Fatalf("second RunSync() state = %+v, want zero value", state)
	}

	close(blockSync)

	var first runSyncResult
	select {
	case first = <-firstDone:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for first sync to finish")
	}

	if !first.message.Empty() {
		t.Fatalf("first RunSync() message = %q, want empty", first.message.Text)
	}
	if svc.syncCallCount() != 1 {
		t.Fatalf("sync calls = %d, want 1", svc.syncCallCount())
	}
}

func TestRunSyncFailureSetsLastErrorCode(t *testing.T) {
	cfg := testConfig()
	championNotSelectedErr := errors.New("champion not selected")

	svc := newStubService()
	svc.syncFn = func(context.Context, autobuild.SyncRequest) (autobuild.SyncResult, error) {
		return autobuild.SyncResult{}, fmt.Errorf("sync: %w", championNotSelectedErr)
	}

	app := newTestApp(t, testAppOptions{
		cfg: cfg,
		serviceFactory: func(RuntimeConfig) (autobuild.Service, error) {
			return svc, nil
		},
		messageFromErr: func(err error) UserMessage {
			if errors.Is(err, championNotSelectedErr) {
				return UserMessage{Code: MessageCodeLCUChampionNotSelected, Text: "Select a champion first."}
			}
			return userMessageFromErr(err)
		},
	})

	state, message := app.RunSync(context.Background())
	if message.Text != "Select a champion first." {
		t.Fatalf("RunSync() message = %q, want %q", message.Text, "Select a champion first.")
	}
	if message.Code != MessageCodeLCUChampionNotSelected {
		t.Fatalf("RunSync() code = %q, want %q", message.Code, MessageCodeLCUChampionNotSelected)
	}
	if state != (ViewState{}) {
		t.Fatalf("RunSync() state = %+v, want zero value", state)
	}

	current := app.State(context.Background())
	if current.LastError != "Select a champion first." {
		t.Fatalf("LastError = %q, want %q", current.LastError, "Select a champion first.")
	}
	if current.LastErrorCode != MessageCodeLCUChampionNotSelected {
		t.Fatalf("LastErrorCode = %q, want %q", current.LastErrorCode, MessageCodeLCUChampionNotSelected)
	}
}

func TestCheckUpdatesAvailable(t *testing.T) {
	checker := &stubUpdateChecker{
		currentVersion: "0.1.0",
		checkFn: func(context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{
				CurrentVersion: "0.1.0",
				LatestVersion:  "v0.2.0",
				DownloadURL:    "https://github.com/controlado/lol-autobuild/releases/tag/v0.2.0",
				Available:      true,
			}, nil
		},
	}
	app := newTestApp(t, testAppOptions{updateChecker: checker})
	app.lastErrorMessage = "sync failed"

	state, message := app.CheckUpdates(context.Background())
	if !message.Empty() {
		t.Fatalf("CheckUpdates() message = %q, want empty", message.Text)
	}
	if checker.callCount() != 1 {
		t.Fatalf("update checker calls = %d, want 1", checker.callCount())
	}
	if state.Update.Status != UpdateStatusAvailable {
		t.Fatalf("Update.Status = %q, want %q", state.Update.Status, UpdateStatusAvailable)
	}
	if state.Update.CurrentVersion != "0.1.0" || state.Update.LatestVersion != "v0.2.0" {
		t.Fatalf("Update versions = %+v", state.Update)
	}
	if state.Update.DownloadURL == "" {
		t.Fatal("expected download URL")
	}
	if state.Update.CheckedAt == nil {
		t.Fatal("expected checked_at")
	}
	if state.LastError != "sync failed" {
		t.Fatalf("LastError = %q, want previous sync error", state.LastError)
	}
}

func TestCheckUpdatesCurrent(t *testing.T) {
	checker := &stubUpdateChecker{
		currentVersion: "0.2.0",
		checkFn: func(context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{
				CurrentVersion: "0.2.0",
				LatestVersion:  "v0.2.0",
				DownloadURL:    "https://example.test/latest",
				Available:      false,
			}, nil
		},
	}
	app := newTestApp(t, testAppOptions{updateChecker: checker})

	state, message := app.CheckUpdates(context.Background())
	if !message.Empty() {
		t.Fatalf("CheckUpdates() message = %q, want empty", message.Text)
	}
	if state.Update.Status != UpdateStatusCurrent {
		t.Fatalf("Update.Status = %q, want %q", state.Update.Status, UpdateStatusCurrent)
	}
	if state.Update.Message == "" {
		t.Fatal("expected update message")
	}
}

func TestCheckUpdatesUnavailableDoesNotSetLastError(t *testing.T) {
	checker := &stubUpdateChecker{
		currentVersion: "dev",
		checkFn: func(context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{CurrentVersion: "dev"}, ErrUpdateUnavailable
		},
	}
	app := newTestApp(t, testAppOptions{updateChecker: checker})
	app.lastErrorMessage = "watch failed"

	state, message := app.CheckUpdates(context.Background())
	if !message.Empty() {
		t.Fatalf("CheckUpdates() message = %q, want empty", message.Text)
	}
	if state.Update.Status != UpdateStatusUnavailable {
		t.Fatalf("Update.Status = %q, want %q", state.Update.Status, UpdateStatusUnavailable)
	}
	if state.LastError != "watch failed" {
		t.Fatalf("LastError = %q, want previous watch error", state.LastError)
	}
}

func TestCheckUpdatesErrorDoesNotSetLastError(t *testing.T) {
	checker := &stubUpdateChecker{
		currentVersion: "0.1.0",
		checkFn: func(context.Context) (UpdateCheckResult, error) {
			return UpdateCheckResult{CurrentVersion: "0.1.0"}, errors.New("github failed")
		},
	}
	app := newTestApp(t, testAppOptions{updateChecker: checker})
	app.lastErrorMessage = "sync failed"

	state, message := app.CheckUpdates(context.Background())
	if !message.Empty() {
		t.Fatalf("CheckUpdates() message = %q, want empty", message.Text)
	}
	if state.Update.Status != UpdateStatusError {
		t.Fatalf("Update.Status = %q, want %q", state.Update.Status, UpdateStatusError)
	}
	if state.Update.Message != "github failed" {
		t.Fatalf("Update.Message = %q, want github failed", state.Update.Message)
	}
	if state.LastError != "sync failed" {
		t.Fatalf("LastError = %q, want previous sync error", state.LastError)
	}
}

func TestCheckUpdatesRejectsConcurrentChecks(t *testing.T) {
	blockCheck := make(chan struct{})
	startedCheck := make(chan struct{})
	checker := &stubUpdateChecker{
		currentVersion: "0.1.0",
		checkFn: func(context.Context) (UpdateCheckResult, error) {
			close(startedCheck)
			<-blockCheck
			return UpdateCheckResult{CurrentVersion: "0.1.0", LatestVersion: "0.1.0"}, nil
		},
	}
	app := newTestApp(t, testAppOptions{updateChecker: checker})

	type updateResult struct {
		state   ViewState
		message UserMessage
	}

	firstDone := make(chan updateResult, 1)
	go func() {
		state, message := app.CheckUpdates(context.Background())
		firstDone <- updateResult{state: state, message: message}
	}()

	waitForSignal(t, startedCheck, "first update check start")

	state, message := app.CheckUpdates(context.Background())
	if !message.Empty() {
		t.Fatalf("second CheckUpdates() message = %q, want empty", message.Text)
	}
	if state.Update.Status != UpdateStatusChecking {
		t.Fatalf("second Update.Status = %q, want %q", state.Update.Status, UpdateStatusChecking)
	}
	if checker.callCount() != 1 {
		t.Fatalf("update checker calls = %d, want 1", checker.callCount())
	}

	close(blockCheck)

	var first updateResult
	select {
	case first = <-firstDone:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for first update check to finish")
	}
	if !first.message.Empty() {
		t.Fatalf("first CheckUpdates() message = %q, want empty", first.message.Text)
	}
	if first.state.Update.Status != UpdateStatusCurrent {
		t.Fatalf("first Update.Status = %q, want %q", first.state.Update.Status, UpdateStatusCurrent)
	}
	if checker.callCount() != 1 {
		t.Fatalf("update checker calls = %d, want 1", checker.callCount())
	}
}

func testConfig() RuntimeConfig {
	return RuntimeConfig{
		Settings: Settings{
			Patch:              "14.7",
			PatchAdditionsMode: autobuild.PatchAdditionsModeAuto,
			PatchAdditions:     autobuild.PatchAdditionsDefault,
			LeagueTierPreset:   autobuild.LeagueTierPresetDefault,
			ApplyItems:         true,
			ApplyRunes:         true,
			ApplySpells:        true,
			KeepFlash:          true,
			DryRun:             true,
			LCUEnabled:         false,
		},
		WatchDebounce: 250 * time.Millisecond,
	}
}

func waitForSyncCall(t *testing.T, svc *stubService) syncCall {
	t.Helper()

	select {
	case call := <-svc.syncCalled:
		return call
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for sync call")
		return syncCall{}
	}
}

func waitForWatchCall(t *testing.T, svc *stubService) watchCall {
	t.Helper()

	select {
	case call := <-svc.watchCalled:
		return call
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for watch call")
		return watchCall{}
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(testTimeout):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func assertSyncRequestMatchesConfig(t *testing.T, got autobuild.SyncRequest, cfg RuntimeConfig) {
	t.Helper()

	if got.Patch != cfg.Settings.Patch {
		t.Fatalf("sync patch = %q, want %q", got.Patch, cfg.Settings.Patch)
	}
	if got.PatchAdditionsMode != cfg.Settings.PatchAdditionsMode {
		t.Fatalf("sync PatchAdditionsMode = %q, want %q", got.PatchAdditionsMode, cfg.Settings.PatchAdditionsMode)
	}
	if got.PatchAdditions != cfg.Settings.PatchAdditions {
		t.Fatalf("sync PatchAdditions = %d, want %d", got.PatchAdditions, cfg.Settings.PatchAdditions)
	}
	if got.LeagueTierPreset != cfg.Settings.LeagueTierPreset {
		t.Fatalf("sync LeagueTierPreset = %q, want %q", got.LeagueTierPreset, cfg.Settings.LeagueTierPreset)
	}
	if got.ApplyItems != cfg.Settings.ApplyItems {
		t.Fatalf("sync ApplyItems = %t, want %t", got.ApplyItems, cfg.Settings.ApplyItems)
	}
	if got.ApplyRunes != cfg.Settings.ApplyRunes {
		t.Fatalf("sync ApplyRunes = %t, want %t", got.ApplyRunes, cfg.Settings.ApplyRunes)
	}
	if got.ApplySpells != cfg.Settings.ApplySpells {
		t.Fatalf("sync ApplySpells = %t, want %t", got.ApplySpells, cfg.Settings.ApplySpells)
	}
	if got.KeepFlash != cfg.Settings.KeepFlash {
		t.Fatalf("sync KeepFlash = %t, want %t", got.KeepFlash, cfg.Settings.KeepFlash)
	}
	if got.DryRun != cfg.Settings.DryRun {
		t.Fatalf("sync DryRun = %t, want %t", got.DryRun, cfg.Settings.DryRun)
	}
}

func assertWatchRequestMatchesConfig(t *testing.T, got autobuild.WatchRequest, cfg RuntimeConfig) {
	t.Helper()

	if got.Patch != cfg.Settings.Patch {
		t.Fatalf("watch patch = %q, want %q", got.Patch, cfg.Settings.Patch)
	}
	if got.PatchAdditionsMode != cfg.Settings.PatchAdditionsMode {
		t.Fatalf("watch PatchAdditionsMode = %q, want %q", got.PatchAdditionsMode, cfg.Settings.PatchAdditionsMode)
	}
	if got.PatchAdditions != cfg.Settings.PatchAdditions {
		t.Fatalf("watch PatchAdditions = %d, want %d", got.PatchAdditions, cfg.Settings.PatchAdditions)
	}
	if got.LeagueTierPreset != cfg.Settings.LeagueTierPreset {
		t.Fatalf("watch LeagueTierPreset = %q, want %q", got.LeagueTierPreset, cfg.Settings.LeagueTierPreset)
	}
	if got.ApplyItems != cfg.Settings.ApplyItems {
		t.Fatalf("watch ApplyItems = %t, want %t", got.ApplyItems, cfg.Settings.ApplyItems)
	}
	if got.ApplyRunes != cfg.Settings.ApplyRunes {
		t.Fatalf("watch ApplyRunes = %t, want %t", got.ApplyRunes, cfg.Settings.ApplyRunes)
	}
	if got.ApplySpells != cfg.Settings.ApplySpells {
		t.Fatalf("watch ApplySpells = %t, want %t", got.ApplySpells, cfg.Settings.ApplySpells)
	}
	if got.KeepFlash != cfg.Settings.KeepFlash {
		t.Fatalf("watch KeepFlash = %t, want %t", got.KeepFlash, cfg.Settings.KeepFlash)
	}
	if got.DryRun != cfg.Settings.DryRun {
		t.Fatalf("watch DryRun = %t, want %t", got.DryRun, cfg.Settings.DryRun)
	}
	wantDebounce := cfg.WatchDebounce
	if got.Debounce != wantDebounce {
		t.Fatalf("watch Debounce = %v, want %v", got.Debounce, wantDebounce)
	}
	if got.OnCycle == nil {
		t.Fatal("watch OnCycle should not be nil")
	}
	if got.OnNotice == nil {
		t.Fatal("watch OnNotice should not be nil")
	}
}

func assertSyncResultEqual(t *testing.T, got *SyncSummary, want *autobuild.SyncResult) {
	t.Helper()

	if !reflect.DeepEqual(got, syncSummaryFromResult(*want)) {
		t.Fatalf("sync result = %+v, want %+v", got, want)
	}
}

func assertTimeBetween(t *testing.T, got *time.Time, before, after time.Time) {
	t.Helper()

	if got == nil {
		t.Fatal("expected time to be set")
	}
	if got.Before(before) || got.After(after) {
		t.Fatalf("time = %v, want between %v and %v", *got, before, after)
	}
}
