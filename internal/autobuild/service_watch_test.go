package autobuild

import (
	"context"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
	"github.com/controlado/lol-autobuild/internal/autobuild/recommend"
)

func TestWatchDoesNotRunStartupSyncAndStopsGracefully(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	lcu := watchTestLCU(func(ctx context.Context, _ chan<- domain.LCUEvent, _ chan<- domain.LCUWatchNotice) error {
		close(started)
		<-ctx.Done()
		return nil
	})

	svc := newWatchTestService(t, lcu)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
			cycleCh <- cycle
		}))
	}()

	select {
	case cycle := <-cycleCh:
		t.Fatalf("unexpected watch cycle before events: %+v", cycle)
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch startup")
	}

	assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)

	cancel()
	waitForWatchExit(t, errCh)

	if lcu.detectCalls != 0 {
		t.Fatalf("expected no sync cycle, got detectCalls=%d", lcu.detectCalls)
	}
}

func TestWatchIgnoresNonFinalizationEventsAndSyncsOnFinalization(t *testing.T) {
	t.Parallel()

	events := make(chan domain.LCUEvent)
	lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
	svc := newWatchTestService(t, lcu)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 2)
	errCh := make(chan error, 1)

	go func() {
		errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
			cycleCh <- cycle
		}))
	}()

	sendWatchEvent(t, events, champSelectSessionEvent("Create", "BAN_PICK"))
	sendWatchEvent(t, events, champSelectSessionEvent("Update", "BAN_PICK"))
	sendWatchEvent(t, events, domain.LCUEvent{EventType: "Update", URI: "/lol-champ-select/v1/session"})
	sendWatchEvent(t, events, domain.LCUEvent{
		EventType: "Create",
		URI:       "/lol-champ-select/v1/grid",
	})

	assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)

	sendWatchEvent(t, events, champSelectSessionEvent("Update", "FINALIZATION"))

	cycle := waitForWatchCycle(t, cycleCh)
	if cycle.Trigger != WatchTriggerEvent {
		t.Fatalf("expected event trigger, got %q", cycle.Trigger)
	}
	if cycle.EventURI != "/lol-champ-select/v1/session" {
		t.Fatalf("expected event uri /lol-champ-select/v1/session, got %q", cycle.EventURI)
	}
	if cycle.EventType != "Update" {
		t.Fatalf("expected finalization event type Update, got %q", cycle.EventType)
	}

	cancel()
	waitForWatchExit(t, errCh)

	if lcu.detectCalls != 1 {
		t.Fatalf("expected one sync cycle, got detectCalls=%d", lcu.detectCalls)
	}
}

func TestWatchRunsOnlyOncePerFinalization(t *testing.T) {
	t.Parallel()

	events := make(chan domain.LCUEvent)
	lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
	svc := newWatchTestService(t, lcu)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 3)
	errCh := make(chan error, 1)

	go func() {
		errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
			cycleCh <- cycle
		}))
	}()

	sendWatchEvent(t, events, champSelectSessionEvent("Create", "FINALIZATION"))
	sendWatchEvent(t, events, champSelectSessionEvent("Update", "FINALIZATION"))

	cycle := waitForWatchCycle(t, cycleCh)
	if cycle.EventType != "Create" {
		t.Fatalf("expected first finalization event to remain pending, got %q", cycle.EventType)
	}

	sendWatchEvent(t, events, champSelectSessionEvent("Update", "FINALIZATION"))
	assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)

	cancel()
	waitForWatchExit(t, errCh)

	if lcu.detectCalls != 1 {
		t.Fatalf("expected one sync cycle, got detectCalls=%d", lcu.detectCalls)
	}
}

func TestWatchResetsFinalizationLockBetweenChampSelects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		resetEvent domain.LCUEvent
	}{
		{
			name:       "delete session",
			resetEvent: domain.LCUEvent{EventType: "Delete", URI: "/lol-champ-select/v1/session"},
		},
		{
			name:       "new non-finalized create",
			resetEvent: champSelectSessionEvent("Create", "BAN_PICK"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan domain.LCUEvent)
			lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
			svc := newWatchTestService(t, lcu)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cycleCh := make(chan WatchCycle, 2)
			errCh := make(chan error, 1)

			go func() {
				errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
					cycleCh <- cycle
				}))
			}()

			sendWatchEvent(t, events, champSelectSessionEvent("Update", "FINALIZATION"))
			_ = waitForWatchCycle(t, cycleCh)

			sendWatchEvent(t, events, tt.resetEvent)
			sendWatchEvent(t, events, champSelectSessionEvent("Update", "FINALIZATION"))
			_ = waitForWatchCycle(t, cycleCh)

			cancel()
			waitForWatchExit(t, errCh)

			if lcu.detectCalls != 2 {
				t.Fatalf("expected two sync cycles after reset, got detectCalls=%d", lcu.detectCalls)
			}
		})
	}
}

func TestWatchReconcilesSnapshotEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		phase     string
		wantSync  bool
		wantCycle WatchTrigger
	}{
		{
			name:      "snapshot finalization runs sync",
			phase:     "FINALIZATION",
			wantSync:  true,
			wantCycle: WatchTriggerSnapshot,
		},
		{
			name:     "snapshot before finalization does not run sync",
			phase:    "BAN_PICK",
			wantSync: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan domain.LCUEvent)
			lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
			svc := newWatchTestService(t, lcu)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cycleCh := make(chan WatchCycle, 1)
			errCh := make(chan error, 1)

			go func() {
				errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
					cycleCh <- cycle
				}))
			}()

			sendWatchEvent(t, events, champSelectSessionEventFromSource("Snapshot", tt.phase, domain.LCUEventSourceSnapshot, 1, "9876"))

			if tt.wantSync {
				cycle := waitForWatchCycle(t, cycleCh)
				if cycle.Trigger != tt.wantCycle {
					t.Fatalf("cycle.Trigger = %q, want %q", cycle.Trigger, tt.wantCycle)
				}
				if cycle.EventType != "Snapshot" {
					t.Fatalf("cycle.EventType = %q, want Snapshot", cycle.EventType)
				}
			} else {
				assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)
			}

			cancel()
			waitForWatchExit(t, errCh)

			wantDetectCalls := 0
			if tt.wantSync {
				wantDetectCalls = 1
			}
			if lcu.detectCalls != wantDetectCalls {
				t.Fatalf("detectCalls = %d, want %d", lcu.detectCalls, wantDetectCalls)
			}
		})
	}
}

func TestWatchSessionGateDeduplicatesMixedGameIDPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		beforeFirstCycle   []domain.LCUEvent
		afterFirstCycle    []domain.LCUEvent
		wantFirstEventType string
	}{
		{
			name: "game id then missing game id",
			beforeFirstCycle: []domain.LCUEvent{
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, "9876"),
			},
			afterFirstCycle: []domain.LCUEvent{
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, ""),
			},
			wantFirstEventType: "Update",
		},
		{
			name: "missing game id then game id after sync",
			beforeFirstCycle: []domain.LCUEvent{
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, ""),
			},
			afterFirstCycle: []domain.LCUEvent{
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, "9876"),
			},
			wantFirstEventType: "Update",
		},
		{
			name: "missing game id promoted while pending",
			beforeFirstCycle: []domain.LCUEvent{
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, ""),
				champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, "9876"),
			},
			wantFirstEventType: "Update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan domain.LCUEvent)
			lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
			svc := newWatchTestService(t, lcu)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cycleCh := make(chan WatchCycle, 2)
			errCh := make(chan error, 1)

			go func() {
				errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
					cycleCh <- cycle
				}))
			}()

			for _, event := range tt.beforeFirstCycle {
				sendWatchEvent(t, events, event)
			}

			cycle := waitForWatchCycle(t, cycleCh)
			if cycle.EventType != tt.wantFirstEventType {
				t.Fatalf("cycle.EventType = %q, want %q", cycle.EventType, tt.wantFirstEventType)
			}

			for _, event := range tt.afterFirstCycle {
				sendWatchEvent(t, events, event)
			}
			assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)

			cancel()
			waitForWatchExit(t, errCh)

			if lcu.detectCalls != 1 {
				t.Fatalf("expected one sync cycle, got detectCalls=%d", lcu.detectCalls)
			}
		})
	}
}

func TestWatchSessionGateAllowsNewFinalizationWindows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		secondEvent domain.LCUEvent
	}{
		{
			name:        "reconnected update without game id",
			secondEvent: champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 2, ""),
		},
		{
			name:        "create finalization starts new anonymous session",
			secondEvent: champSelectSessionEventFromSource("Create", "FINALIZATION", domain.LCUEventSourceStream, 1, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan domain.LCUEvent)
			lcu := watchTestLCU(watchEventsWithNoticesFrom(events))
			svc := newWatchTestService(t, lcu)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cycleCh := make(chan WatchCycle, 2)
			errCh := make(chan error, 1)

			go func() {
				errCh <- svc.Watch(ctx, watchTestRequest(func(cycle WatchCycle) {
					cycleCh <- cycle
				}))
			}()

			sendWatchEvent(t, events, champSelectSessionEventFromSource("Update", "FINALIZATION", domain.LCUEventSourceStream, 1, ""))
			_ = waitForWatchCycle(t, cycleCh)

			sendWatchEvent(t, events, tt.secondEvent)
			_ = waitForWatchCycle(t, cycleCh)

			cancel()
			waitForWatchExit(t, errCh)

			if lcu.detectCalls != 2 {
				t.Fatalf("expected two sync cycles, got detectCalls=%d", lcu.detectCalls)
			}
		})
	}
}

func TestWatchForwardsSnapshotUnavailableNotice(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	lcu := watchTestLCU(nil)
	lcu.watchEventsWithNoticesFn = func(ctx context.Context, _ chan<- domain.LCUEvent, notices chan<- domain.LCUWatchNotice) error {
		close(started)
		select {
		case notices <- domain.LCUWatchNotice{
			Kind:    domain.LCUWatchNoticeSnapshotWaiting,
			Message: "snapshot unavailable",
			Err:     context.DeadlineExceeded,
			URI:     champSelectSessionURI,
		}:
		case <-ctx.Done():
			return nil
		}
		<-ctx.Done()
		return nil
	}

	svc := newWatchTestService(t, lcu)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 1)
	noticeCh := make(chan WatchNotice, 1)
	errCh := make(chan error, 1)
	req := watchTestRequest(func(cycle WatchCycle) {
		cycleCh <- cycle
	})
	req.OnNotice = func(notice WatchNotice) {
		noticeCh <- notice
	}

	go func() {
		errCh <- svc.Watch(ctx, req)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch startup")
	}

	select {
	case notice := <-noticeCh:
		if notice.Kind != WatchNoticeSnapshotWaiting {
			t.Fatalf("notice.Kind = %q, want %q", notice.Kind, WatchNoticeSnapshotWaiting)
		}
		if notice.Err == nil {
			t.Fatal("expected notice error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch notice")
	}

	assertNoWatchCycle(t, cycleCh, 80*time.Millisecond)

	cancel()
	waitForWatchExit(t, errCh)
}

func newWatchTestService(t *testing.T, lcu *lcuStub) Service {
	t.Helper()

	svc, err := NewService(ServiceDeps{
		Coachless:   &coachlessStub{},
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return svc
}

func watchTestLCU(watcher func(context.Context, chan<- domain.LCUEvent, chan<- domain.LCUWatchNotice) error) *lcuStub {
	return &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Mid,
			QueueID:    420,
		},
		watchEventsWithNoticesFn: watcher,
	}
}

func watchTestRequest(onCycle func(WatchCycle)) WatchRequest {
	return WatchRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		KeepFlash:   true,
		DryRun:      true,
		Debounce:    20 * time.Millisecond,
		OnCycle:     onCycle,
	}
}

func watchEventsWithNoticesFrom(events <-chan domain.LCUEvent) func(context.Context, chan<- domain.LCUEvent, chan<- domain.LCUWatchNotice) error {
	return func(ctx context.Context, out chan<- domain.LCUEvent, _ chan<- domain.LCUWatchNotice) error {
		for {
			select {
			case event := <-events:
				select {
				case out <- event:
				case <-ctx.Done():
					return nil
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func champSelectSessionEvent(eventType string, phase string) domain.LCUEvent {
	return champSelectSessionEventFromSource(eventType, phase, "", 0, "")
}

func champSelectSessionEventFromSource(eventType string, phase string, source domain.LCUEventSource, connectionID int, gameID string) domain.LCUEvent {
	return domain.LCUEvent{
		EventType:        eventType,
		URI:              "/lol-champ-select/v1/session",
		Source:           source,
		ConnectionID:     connectionID,
		ChampSelectPhase: phase,
		GameID:           gameID,
	}
}

func sendWatchEvent(t *testing.T, events chan<- domain.LCUEvent, event domain.LCUEvent) {
	t.Helper()

	select {
	case events <- event:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out sending watch event %+v", event)
	}
}

func waitForWatchCycle(t *testing.T, cycleCh <-chan WatchCycle) WatchCycle {
	t.Helper()

	select {
	case cycle := <-cycleCh:
		return cycle
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch cycle")
		return WatchCycle{}
	}
}

func assertNoWatchCycle(t *testing.T, cycleCh <-chan WatchCycle, duration time.Duration) {
	t.Helper()

	select {
	case cycle := <-cycleCh:
		t.Fatalf("unexpected watch cycle: %+v", cycle)
	case <-time.After(duration):
	}
}

func waitForWatchExit(t *testing.T, errCh <-chan error) {
	t.Helper()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch exit")
	}
}
