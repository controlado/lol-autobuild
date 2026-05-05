package app

import (
	"encoding/json"
	"testing"
	"time"
)

func TestViewStateJSONContract(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	authExpiresAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	state := ViewState{
		Settings: Settings{
			Patch:      "15.1",
			ApplyRunes: true,
			LCUEnabled: true,
		},
		LCU:     LCUStatus{State: LCUConnectionStateConnected},
		Watcher: WatcherState{ConfigStale: true},
		CoachlessAuth: CoachlessAuthState{
			Status:    CoachlessAuthStatusStored,
			Plan:      CoachlessAuthPlanPremium,
			ExpiresAt: &authExpiresAt,
		},
		Update: UpdateState{
			DownloadURL: "https://example.test/download",
			CheckedAt:   &checkedAt,
		},
		LastSync: &SyncSummary{
			DetectedChampionID:   22,
			DetectedChampionName: "Ashe",
			Warnings:             []string{"warning"},
		},
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	settings := objectAt(t, got, "settings")
	if settings["apply_runes"] != true || settings["lcu_enabled"] != true {
		t.Fatalf("settings JSON = %+v", settings)
	}

	lcu := objectAt(t, got, "lcu")
	if lcu["state"] != string(LCUConnectionStateConnected) {
		t.Fatalf("lcu.state = %v, want %q", lcu["state"], LCUConnectionStateConnected)
	}

	watcher := objectAt(t, got, "watcher")
	if watcher["config_stale"] != true {
		t.Fatalf("watcher.config_stale = %v, want true", watcher["config_stale"])
	}

	coachlessAuth := objectAt(t, got, "coachless_auth")
	if coachlessAuth["status"] != string(CoachlessAuthStatusStored) || coachlessAuth["plan"] != string(CoachlessAuthPlanPremium) {
		t.Fatalf("coachless_auth JSON = %+v", coachlessAuth)
	}

	update := objectAt(t, got, "update")
	if update["download_url"] != "https://example.test/download" {
		t.Fatalf("update.download_url = %v", update["download_url"])
	}

	lastSync := objectAt(t, got, "last_sync")
	if lastSync["DetectedChampionID"] != float64(22) {
		t.Fatalf("last_sync.DetectedChampionID = %v, want 22", lastSync["DetectedChampionID"])
	}
	if lastSync["DetectedChampionName"] != "Ashe" {
		t.Fatalf("last_sync.DetectedChampionName = %v, want Ashe", lastSync["DetectedChampionName"])
	}
	warnings, ok := lastSync["Warnings"].([]any)
	if !ok || len(warnings) != 1 || warnings[0] != "warning" {
		t.Fatalf("last_sync.Warnings = %#v", lastSync["Warnings"])
	}
}

func objectAt(t *testing.T, values map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := values[key]
	if !ok {
		t.Fatalf("missing JSON key %q in %+v", key, values)
	}

	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON key %q = %#v, want object", key, value)
	}
	return out
}
