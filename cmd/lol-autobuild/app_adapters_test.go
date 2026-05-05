package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/app"
	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/autobuild"
	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/internal/update"
)

type recordingConfigSaver struct {
	saved []config.Config
}

func (s *recordingConfigSaver) Save(cfg config.Config) error {
	s.saved = append(s.saved, cfg)
	return nil
}

func TestRuntimeConfigConversions(t *testing.T) {
	t.Parallel()

	base := config.Defaults()
	base.Coachless.APIBaseURL = "https://api.example.test"
	base.LCU.LockfilePath = "lockfile"
	base.Watch.DebounceMillis = 777
	base.Watch.ReconnectDelayMillis = 1234
	base.Sync.Patch = "14.7"
	base.Sync.ApplyRunes = false

	appCfg := runtimeConfigFromConfig(base)
	if appCfg.Settings.Patch != "14.7" || appCfg.Settings.ApplyRunes {
		t.Fatalf("runtime config settings = %+v", appCfg.Settings)
	}
	if appCfg.WatchDebounce != 777*time.Millisecond {
		t.Fatalf("runtime config debounce = %v, want 777ms", appCfg.WatchDebounce)
	}

	appCfg.Settings = app.Settings{
		Patch:              "15.1",
		PatchAdditionsMode: autobuild.PatchAdditionsModeManual,
		PatchAdditions:     4,
		LeagueTierPreset:   autobuild.LeagueTierPresetMasterPlus,
		ApplyItems:         false,
		ApplyRunes:         true,
		ApplySpells:        false,
		KeepFlash:          false,
		DryRun:             true,
		LCUEnabled:         true,
	}
	appCfg.WatchDebounce = 350 * time.Millisecond

	got := configFromRuntimeConfig(base, appCfg)
	if got.Sync.Patch != "15.1" || got.Sync.PatchAdditions != 4 || got.Sync.LeagueTierPreset != autobuild.LeagueTierPresetMasterPlus {
		t.Fatalf("converted sync config = %+v", got.Sync)
	}
	if !got.LCU.Enabled || got.LCU.LockfilePath != "lockfile" {
		t.Fatalf("converted LCU config = %+v", got.LCU)
	}
	if got.Watch.DebounceMillis != 350 || got.Watch.ReconnectDelayMillis != 1234 {
		t.Fatalf("converted watch config = %+v", got.Watch)
	}
	if got.Coachless.APIBaseURL != "https://api.example.test" {
		t.Fatalf("coachless config changed: %+v", got.Coachless)
	}
}

func TestAppConfigStoreSavesMergedConfigAndUpdatesBase(t *testing.T) {
	t.Parallel()

	base := config.Defaults()
	base.LCU.LockfilePath = "persisted-lockfile"
	saver := &recordingConfigSaver{}
	store := newAppConfigStore(saver, base)

	first := runtimeConfigFromConfig(base)
	first.Settings.Patch = "15.1"
	first.Settings.LCUEnabled = true
	first.WatchDebounce = 300 * time.Millisecond

	if err := store.Save(first); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if len(saver.saved) != 1 {
		t.Fatalf("saved configs = %d, want 1", len(saver.saved))
	}
	if saver.saved[0].Sync.Patch != "15.1" || !saver.saved[0].LCU.Enabled || saver.saved[0].Watch.DebounceMillis != 300 {
		t.Fatalf("saved config = %+v", saver.saved[0])
	}

	second := first
	second.Settings.Patch = "15.2"
	second.WatchDebounce = 400 * time.Millisecond
	got := store.configFor(second)
	if got.Sync.Patch != "15.2" || got.Watch.DebounceMillis != 400 {
		t.Fatalf("configFor() = %+v", got)
	}
	if got.LCU.LockfilePath != "persisted-lockfile" {
		t.Fatalf("configFor() lost base LCU fields: %+v", got.LCU)
	}
}

func TestAppLCUStatusFromLCU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   lcu.ConnectionStatus
		want app.LCUStatus
	}{
		{
			name: "off",
			in:   lcu.ConnectionStatus{State: lcu.ConnectionStateOff, Message: "LCU is off"},
			want: app.LCUStatus{State: app.LCUConnectionStateOff, Message: "LCU is off"},
		},
		{
			name: "connected",
			in:   lcu.ConnectionStatus{State: lcu.ConnectionStateConnected, Source: "lockfile"},
			want: app.LCUStatus{State: app.LCUConnectionStateConnected, Source: "lockfile"},
		},
		{
			name: "not connected",
			in:   lcu.ConnectionStatus{State: lcu.ConnectionStateNotConnected, Message: "missing"},
			want: app.LCUStatus{State: app.LCUConnectionStateNotConnected, Message: "missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := appLCUStatusFromLCU(tt.in); got != tt.want {
				t.Fatalf("appLCUStatusFromLCU() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAppCoachlessAuthStateFromAuth(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC)
	got := appCoachlessAuthStateFromAuth(auth.CoachlessSessionState{
		Status:    auth.CoachlessSessionStatusStored,
		Plan:      auth.CoachlessPlanPremium,
		ExpiresAt: &expiresAt,
		Message:   "status",
	})

	if got.Status != app.CoachlessAuthStatusStored || got.Plan != app.CoachlessAuthPlanPremium {
		t.Fatalf("converted status = %+v", got)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("converted ExpiresAt = %v, want %v", got.ExpiresAt, expiresAt)
	}
	if got.Message != "status" {
		t.Fatalf("converted Message = %q, want status", got.Message)
	}
}

type stubUpdateSource struct {
	currentVersion string
	result         update.Result
	err            error
}

func (s *stubUpdateSource) CurrentVersion() string {
	return s.currentVersion
}

func (s *stubUpdateSource) Check(context.Context) (update.Result, error) {
	return s.result, s.err
}

func TestAppUpdateCheckerConvertsResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  stubUpdateSource
		wantErr error
	}{
		{
			name: "available",
			source: stubUpdateSource{
				currentVersion: "0.1.0",
				result:         update.Result{CurrentVersion: "0.1.0", LatestVersion: "v0.2.0", DownloadURL: "https://example.test", Available: true},
			},
		},
		{
			name: "unavailable",
			source: stubUpdateSource{
				currentVersion: "dev",
				result:         update.Result{CurrentVersion: "dev"},
				err:            update.ErrUnavailable,
			},
			wantErr: app.ErrUpdateUnavailable,
		},
		{
			name: "raw error",
			source: stubUpdateSource{
				currentVersion: "0.1.0",
				result:         update.Result{CurrentVersion: "0.1.0"},
				err:            errors.New("github failed"),
			},
			wantErr: errors.New("github failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := appUpdateChecker{source: &tt.source}
			got, err := checker.Check(context.Background())
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Check() error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
				t.Fatalf("Check() error = %v, want %v", err, tt.wantErr)
			}
			if got.CurrentVersion != tt.source.result.CurrentVersion || got.Available != tt.source.result.Available {
				t.Fatalf("Check() result = %+v, want source %+v", got, tt.source.result)
			}
			if checker.CurrentVersion() != tt.source.currentVersion {
				t.Fatalf("CurrentVersion() = %q, want %q", checker.CurrentVersion(), tt.source.currentVersion)
			}
		})
	}
}

func TestAppMessageFromErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want app.UserMessage
	}{
		{
			name: "nil",
			err:  nil,
			want: app.UserMessage{},
		},
		{
			name: "lcu off",
			err:  fmt.Errorf("sync: %w", lcu.ErrNotConfigured),
			want: app.UserMessage{Code: app.MessageCodeLCUOff, Text: "LCU is off."},
		},
		{
			name: "lockfile missing",
			err:  fmt.Errorf("sync: %w", lcu.ErrLockfileNotFound),
			want: app.UserMessage{Code: app.MessageCodeLCULockfileNotFound, Text: "League Client is not open."},
		},
		{
			name: "champ select unavailable",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampSelectUnavailable),
			want: app.UserMessage{Code: app.MessageCodeLCUChampSelectUnavailable, Text: "Champ select is not ready."},
		},
		{
			name: "champion not selected",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampionNotSelected),
			want: app.UserMessage{Code: app.MessageCodeLCUChampionNotSelected, Text: "Select a champion first."},
		},
		{
			name: "coachless token error",
			err:  fmt.Errorf("auth: %w", auth.ErrAccessTokenUnavailable),
			want: app.UserMessage{Code: app.MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."},
		},
		{
			name: "fallback",
			err:  errors.New("unexpected failure"),
			want: app.UserMessage{Text: "unexpected failure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := appMessageFromErr(tt.err); got != tt.want {
				t.Fatalf("appMessageFromErr() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
