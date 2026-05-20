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
			Regions:    []int{0, 8},
			ApplyRunes: true,
			LCUEnabled: true,
		},
		CoachlessRegions: []CoachlessRegionOption{
			{ID: 0, Label: "BR"},
			{ID: 8, Label: "NA"},
		},
		LCU: LCUStatus{State: LCUConnectionStateNotConnected, Message: NewMessageDescriptor("lcu.not_reachable", "League Client is not reachable.")},
		Watcher: WatcherState{
			ConfigStale: true,
			LastNotice: &WatcherNoticeState{
				Kind:    "reconnecting",
				Message: NewMessageDescriptor("watch.notice.reconnecting", "reconnecting"),
				Error:   NewMessageDescriptor("", "socket closed"),
				At:      checkedAt,
			},
		},
		CoachlessAuth: CoachlessAuthState{
			Status:    CoachlessAuthStatusStored,
			Plan:      CoachlessAuthPlanPremium,
			ExpiresAt: &authExpiresAt,
			Message:   NewMessageDescriptor("", "stored"),
		},
		Update: UpdateState{
			DownloadURL: "https://example.test/download",
			CheckedAt:   &checkedAt,
			Message:     NewMessageDescriptor("update.up_to_date", "You have the latest version."),
		},
		LastSync: &SyncSummary{
			DetectedChampionID:   22,
			DetectedChampionName: "Ashe",
			Warnings: []MessageDescriptor{
				{Key: "warning.key", Fallback: "warning"},
			},
		},
		LastError: NewMessageDescriptor("sync.failed", "Sync failed."),
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
	regions, ok := settings["regions"].([]any)
	if !ok || len(regions) != 2 || regions[0] != float64(0) || regions[1] != float64(8) {
		t.Fatalf("settings.regions = %#v", settings["regions"])
	}
	if settings["apply_runes"] != true || settings["lcu_enabled"] != true {
		t.Fatalf("settings JSON = %+v", settings)
	}

	coachlessRegions, ok := got["coachless_regions"].([]any)
	if !ok || len(coachlessRegions) != 2 {
		t.Fatalf("coachless_regions = %#v", got["coachless_regions"])
	}
	region := objectValueAt(t, coachlessRegions[1], "coachless_regions[1]")
	if region["id"] != float64(8) || region["label"] != "NA" {
		t.Fatalf("coachless_regions[1] = %#v", region)
	}

	lcu := objectAt(t, got, "lcu")
	if lcu["state"] != string(LCUConnectionStateNotConnected) {
		t.Fatalf("lcu.state = %v, want %q", lcu["state"], LCUConnectionStateNotConnected)
	}
	assertJSONDescriptor(t, objectAt(t, lcu, "message"), "lcu.not_reachable", "League Client is not reachable.")

	watcher := objectAt(t, got, "watcher")
	if watcher["config_stale"] != true {
		t.Fatalf("watcher.config_stale = %v, want true", watcher["config_stale"])
	}
	notice := objectAt(t, watcher, "last_notice")
	assertJSONDescriptor(t, objectAt(t, notice, "message"), "watch.notice.reconnecting", "reconnecting")
	assertJSONDescriptor(t, objectAt(t, notice, "error"), "", "socket closed")

	coachlessAuth := objectAt(t, got, "coachless_auth")
	if coachlessAuth["status"] != string(CoachlessAuthStatusStored) || coachlessAuth["plan"] != string(CoachlessAuthPlanPremium) {
		t.Fatalf("coachless_auth JSON = %+v", coachlessAuth)
	}
	assertJSONDescriptor(t, objectAt(t, coachlessAuth, "message"), "", "stored")

	update := objectAt(t, got, "update")
	if update["download_url"] != "https://example.test/download" {
		t.Fatalf("update.download_url = %v", update["download_url"])
	}
	assertJSONDescriptor(t, objectAt(t, update, "message"), "update.up_to_date", "You have the latest version.")

	lastSync := objectAt(t, got, "last_sync")
	if lastSync["DetectedChampionID"] != float64(22) {
		t.Fatalf("last_sync.DetectedChampionID = %v, want 22", lastSync["DetectedChampionID"])
	}
	if lastSync["DetectedChampionName"] != "Ashe" {
		t.Fatalf("last_sync.DetectedChampionName = %v, want Ashe", lastSync["DetectedChampionName"])
	}
	warnings, ok := lastSync["Warnings"].([]any)
	if !ok || len(warnings) != 1 {
		t.Fatalf("last_sync.Warnings = %#v", lastSync["Warnings"])
	}
	warning, ok := warnings[0].(map[string]any)
	if !ok || warning["key"] != "warning.key" || warning["fallback"] != "warning" {
		t.Fatalf("last_sync.Warnings[0] = %#v", warnings[0])
	}
	assertJSONDescriptor(t, objectAt(t, got, "last_error"), "sync.failed", "Sync failed.")
	if _, ok := got["last_error_code"]; ok {
		t.Fatalf("last_error_code = %v, want omitted", got["last_error_code"])
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

func objectValueAt(t *testing.T, value any, label string) map[string]any {
	t.Helper()

	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", label, value)
	}
	return out
}

func assertJSONDescriptor(t *testing.T, got map[string]any, wantKey, wantFallback string) {
	t.Helper()

	if stringValue(got["key"]) != wantKey {
		t.Fatalf("descriptor key = %v, want %q", got["key"], wantKey)
	}
	if stringValue(got["fallback"]) != wantFallback {
		t.Fatalf("descriptor fallback = %v, want %q", got["fallback"], wantFallback)
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	out, _ := value.(string)
	return out
}
