package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/app"
	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/internal/update"
)

type configSaver interface {
	Save(config.Config) error
}

type appConfigStore struct {
	mu    sync.Mutex
	store configSaver
	cfg   config.Config
}

func newAppConfigStore(store configSaver, cfg config.Config) *appConfigStore {
	return &appConfigStore{
		store: store,
		cfg:   cfg,
	}
}

func (s *appConfigStore) Save(newCfg app.RuntimeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := configFromRuntimeConfig(s.cfg, newCfg)
	if err := s.store.Save(next); err != nil {
		return err
	}

	s.cfg = next
	return nil
}

func (s *appConfigStore) configFor(appCfg app.RuntimeConfig) config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	return configFromRuntimeConfig(s.cfg, appCfg)
}

func configFromRuntimeConfig(base config.Config, appCfg app.RuntimeConfig) config.Config {
	base.Sync = config.SyncConfig{
		Patch:              appCfg.Settings.Patch,
		PatchAdditionsMode: appCfg.Settings.PatchAdditionsMode,
		PatchAdditions:     appCfg.Settings.PatchAdditions,
		LeagueTierPreset:   appCfg.Settings.LeagueTierPreset,
		ApplyItems:         appCfg.Settings.ApplyItems,
		ApplyRunes:         appCfg.Settings.ApplyRunes,
		ApplySpells:        appCfg.Settings.ApplySpells,
		KeepFlash:          appCfg.Settings.KeepFlash,
		DryRun:             appCfg.Settings.DryRun,
	}
	base.LCU.Enabled = appCfg.Settings.LCUEnabled
	base.Watch.DebounceMillis = int(appCfg.WatchDebounce / time.Millisecond)
	return base
}

func runtimeConfigFromConfig(cfg config.Config) app.RuntimeConfig {
	return app.RuntimeConfig{
		Settings: app.Settings{
			Patch:              cfg.Sync.Patch,
			PatchAdditionsMode: cfg.Sync.PatchAdditionsMode,
			PatchAdditions:     cfg.Sync.PatchAdditions,
			LeagueTierPreset:   cfg.Sync.LeagueTierPreset,
			ApplyItems:         cfg.Sync.ApplyItems,
			ApplyRunes:         cfg.Sync.ApplyRunes,
			ApplySpells:        cfg.Sync.ApplySpells,
			KeepFlash:          cfg.Sync.KeepFlash,
			DryRun:             cfg.Sync.DryRun,
			LCUEnabled:         cfg.LCU.Enabled,
		},
		WatchDebounce: time.Duration(cfg.Watch.DebounceMillis) * time.Millisecond,
	}
}

func appLCUStatusFromLCU(status lcu.ConnectionStatus) app.LCUStatus {
	return app.LCUStatus{
		State:   appLCUConnectionStateFromLCU(status.State),
		Message: appLCUStatusMessageFromLCU(status),
		Source:  status.Source,
	}
}

func appLCUStatusMessageFromLCU(status lcu.ConnectionStatus) string {
	switch {
	case status.State == lcu.ConnectionStateOff:
		return app.MessageCodeLCUOff
	case status.State == lcu.ConnectionStateConnected:
		return ""
	case errors.Is(status.Err, lcu.ErrLCUNotReachable),
		errors.Is(status.Err, context.DeadlineExceeded):
		return app.MessageCodeLCUNotReachable
	case errors.Is(status.Err, lcu.ErrLockfileNotFound):
		return app.MessageCodeLCULockfileNotFound
	default:
		return app.MessageCodeLCUNotConnected
	}
}

func appLCUConnectionStateFromLCU(state lcu.ConnectionState) app.LCUConnectionState {
	switch state {
	case lcu.ConnectionStateOff:
		return app.LCUConnectionStateOff
	case lcu.ConnectionStateConnected:
		return app.LCUConnectionStateConnected
	default:
		return app.LCUConnectionStateNotConnected
	}
}

type updateSource interface {
	CurrentVersion() string
	Check(context.Context) (update.Result, error)
}

type appUpdateChecker struct {
	source updateSource
}

func (c appUpdateChecker) CurrentVersion() string {
	return c.source.CurrentVersion()
}

func (c appUpdateChecker) Check(ctx context.Context) (app.UpdateCheckResult, error) {
	result, err := c.source.Check(ctx)
	out := app.UpdateCheckResult{
		CurrentVersion: result.CurrentVersion,
		LatestVersion:  result.LatestVersion,
		DownloadURL:    result.DownloadURL,
		Available:      result.Available,
	}
	if errors.Is(err, update.ErrUnavailable) {
		return out, app.ErrUpdateUnavailable
	}

	return out, err
}

type appCoachlessAuthSession struct {
	session *auth.CoachlessSession
}

func (s appCoachlessAuthSession) Status(ctx context.Context) app.CoachlessAuthState {
	if s.session == nil {
		return app.CoachlessAuthState{
			Status: app.CoachlessAuthStatusMissing,
			Plan:   app.CoachlessAuthPlanUnknown,
		}
	}

	return appCoachlessAuthStateFromAuth(s.session.Status(ctx))
}

func (s appCoachlessAuthSession) Login(ctx context.Context) error {
	return s.session.Login(ctx)
}

func (s appCoachlessAuthSession) Logout(ctx context.Context) error {
	return s.session.Logout(ctx)
}

func appCoachlessAuthStateFromAuth(status auth.CoachlessSessionState) app.CoachlessAuthState {
	return app.CoachlessAuthState{
		Status:    appCoachlessAuthStatusFromAuth(status.Status),
		Plan:      appCoachlessAuthPlanFromAuth(status.Plan),
		ExpiresAt: status.ExpiresAt,
		Message:   status.Message,
	}
}

func appCoachlessAuthStatusFromAuth(status auth.CoachlessSessionStatus) app.CoachlessAuthStatus {
	switch status {
	case auth.CoachlessSessionStatusStored:
		return app.CoachlessAuthStatusStored
	case auth.CoachlessSessionStatusExpired:
		return app.CoachlessAuthStatusExpired
	case auth.CoachlessSessionStatusError:
		return app.CoachlessAuthStatusError
	default:
		return app.CoachlessAuthStatusMissing
	}
}

func appCoachlessAuthPlanFromAuth(plan auth.CoachlessPlan) app.CoachlessAuthPlan {
	switch plan {
	case auth.CoachlessPlanFree:
		return app.CoachlessAuthPlanFree
	case auth.CoachlessPlanPremium:
		return app.CoachlessAuthPlanPremium
	default:
		return app.CoachlessAuthPlanUnknown
	}
}

func appMessageFromErr(err error) app.UserMessage {
	switch {
	case err == nil:
		return app.UserMessage{}
	case errors.Is(err, lcu.ErrNotConfigured):
		return app.UserMessage{Code: app.MessageCodeLCUOff, Text: "LCU is off."}
	case errors.Is(err, lcu.ErrLockfileNotFound):
		return app.UserMessage{Code: app.MessageCodeLCULockfileNotFound, Text: "League Client is not open."}
	case errors.Is(err, lcu.ErrLCUNotReachable):
		return app.UserMessage{Code: app.MessageCodeLCUNotReachable, Text: "League Client is not reachable."}
	case errors.Is(err, lcu.ErrChampSelectUnavailable):
		return app.UserMessage{Code: app.MessageCodeLCUChampSelectUnavailable, Text: "Champ select is not ready."}
	case errors.Is(err, lcu.ErrChampionNotSelected):
		return app.UserMessage{Code: app.MessageCodeLCUChampionNotSelected, Text: "Select a champion first."}
	case errors.Is(err, auth.ErrAccessTokenUnavailable):
		return app.UserMessage{Code: app.MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."}
	default:
		return app.UserMessage{Text: err.Error()}
	}
}
