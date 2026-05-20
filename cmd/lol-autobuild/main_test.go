package main

import (
	"slices"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild"
	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/spf13/cobra"
)

func TestRunCommandsUseDryRunFlagDefaultFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{name: "sync", cmd: syncCmd},
		{name: "watch", cmd: watchCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.cmd()
			flag := cmd.Flags().Lookup("dry-run")
			if flag == nil {
				t.Fatal("dry-run flag is missing")
			}
			if flag.DefValue != "false" {
				t.Fatalf("dry-run default = %q, want %q", flag.DefValue, "false")
			}

			got, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				t.Fatalf("get dry-run flag: %v", err)
			}
			if got {
				t.Fatal("dry-run flag value = true, want false")
			}
		})
	}
}

func TestSubcommandsRejectPositionalArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{name: "ui", cmd: uiCmd},
		{name: "sync", cmd: syncCmd},
		{name: "watch", cmd: watchCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.cmd()
			if err := cmd.Args(cmd, []string{"extra"}); err == nil {
				t.Fatal("Args() error = nil, want error for positional arg")
			}
		})
	}
}

func TestSyncRequestFromConfigAndFlagsUsesConfigByDefault(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Sync.Patch = "16.7"
	cfg.Sync.PatchAdditionsMode = autobuild.PatchAdditionsModeManual
	cfg.Sync.PatchAdditions = 4
	cfg.Sync.LeagueTierPreset = autobuild.LeagueTierPresetMasterPlus
	cfg.Sync.Regions = []int{autobuild.CoachlessRegionBR, autobuild.CoachlessRegionNA}
	cfg.Sync.ApplyItems = false
	cfg.Sync.ApplyRunes = false
	cfg.Sync.ApplySpells = true
	cfg.Sync.KeepFlash = false
	cfg.Sync.DryRun = false

	got := syncRequestFromConfigAndFlags(cfg, executionFlags{
		Patch:       "ignored",
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: false,
		DryRun:      true,
	}, executionFlagChanges{})

	if got.Patch != "16.7" || got.PatchAdditionsMode != autobuild.PatchAdditionsModeManual || got.PatchAdditions != 4 || got.LeagueTierPreset != autobuild.LeagueTierPresetMasterPlus {
		t.Fatalf("advanced sync config = %+v", got)
	}
	if !slices.Equal(got.Regions, []int{autobuild.CoachlessRegionBR, autobuild.CoachlessRegionNA}) {
		t.Fatalf("regions = %+v", got.Regions)
	}
	if got.ApplyItems || got.ApplyRunes || !got.ApplySpells || got.KeepFlash || got.DryRun {
		t.Fatalf("sync booleans = %+v", got)
	}
}

func TestSyncRequestFromConfigAndFlagsAppliesExplicitOverrides(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Sync.Patch = "16.7"
	cfg.Sync.ApplyItems = true
	cfg.Sync.ApplyRunes = false
	cfg.Sync.ApplySpells = true
	cfg.Sync.DryRun = false

	got := syncRequestFromConfigAndFlags(cfg, executionFlags{
		Patch:       "16.9",
		ApplyItems:  false,
		ApplyRunes:  true,
		ApplySpells: false,
		DryRun:      true,
	}, executionFlagChanges{
		Patch:       true,
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      true,
	})

	if got.Patch != "16.9" || got.ApplyItems || !got.ApplyRunes || got.ApplySpells || !got.DryRun {
		t.Fatalf("sync request = %+v", got)
	}
}

func TestWatchRequestFromConfigAndFlagsUsesWatchConfig(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Watch.DebounceMillis = 750
	cfg.Sync.KeepFlash = false
	cfg.Sync.PatchAdditions = 3
	cfg.Sync.Regions = []int{autobuild.CoachlessRegionEUW}

	got := watchRequestFromConfigAndFlags(cfg, executionFlags{}, executionFlagChanges{})
	if got.Debounce != 750*time.Millisecond {
		t.Fatalf("watch debounce = %v, want 750ms", got.Debounce)
	}
	if got.KeepFlash || got.PatchAdditions != 3 || !slices.Equal(got.Regions, []int{autobuild.CoachlessRegionEUW}) {
		t.Fatalf("watch request = %+v", got)
	}
}

func TestExecutionFlagChangesFromCommandOnlyMarksExplicitFlags(t *testing.T) {
	t.Parallel()

	cmd := syncCmd()
	if err := cmd.Flags().Set("patch", "16.9"); err != nil {
		t.Fatalf("set patch: %v", err)
	}
	if err := cmd.Flags().Set("apply-items", "false"); err != nil {
		t.Fatalf("set apply-items: %v", err)
	}

	got := executionFlagChangesFromCommand(cmd)
	if !got.Patch || !got.ApplyItems {
		t.Fatalf("explicit changes not detected: %+v", got)
	}
	if got.ApplyRunes || got.ApplySpells || got.DryRun {
		t.Fatalf("implicit defaults marked changed: %+v", got)
	}
}
