package lolautobuild

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/position"
	"github.com/controlado/lol-autobuild/internal/recommend"
)

func TestWatchDoesNotRunStartupSyncAndStopsGracefully(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	lcu := watchTestLCU(func(ctx context.Context, out chan<- ports.LCUEvent) error {
		_ = out
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

	events := make(chan ports.LCUEvent)
	lcu := watchTestLCU(watchEventsFrom(events))
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
	sendWatchEvent(t, events, ports.LCUEvent{EventType: "Update", URI: "/lol-champ-select/v1/session"})
	sendWatchEvent(t, events, ports.LCUEvent{
		EventType: "Create",
		URI:       "/lol-champ-select/v1/grid",
		Data:      json.RawMessage(`{"timer":{"phase":"FINALIZATION"}}`),
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

	events := make(chan ports.LCUEvent)
	lcu := watchTestLCU(watchEventsFrom(events))
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
		resetEvent ports.LCUEvent
	}{
		{
			name:       "delete session",
			resetEvent: ports.LCUEvent{EventType: "Delete", URI: "/lol-champ-select/v1/session"},
		},
		{
			name:       "new non-finalized create",
			resetEvent: champSelectSessionEvent("Create", "BAN_PICK"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan ports.LCUEvent)
			lcu := watchTestLCU(watchEventsFrom(events))
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

func watchTestLCU(watchEventsFn func(context.Context, chan<- ports.LCUEvent) error) *lcuStub {
	return &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID: 240,
			Position:   position.Mid,
			QueueID:    420,
		},
		watchEventsFn: watchEventsFn,
	}
}

func watchTestRequest(onCycle func(WatchCycle)) WatchRequest {
	return WatchRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      true,
		Debounce:    20 * time.Millisecond,
		OnCycle:     onCycle,
	}
}

func watchEventsFrom(events <-chan ports.LCUEvent) func(context.Context, chan<- ports.LCUEvent) error {
	return func(ctx context.Context, out chan<- ports.LCUEvent) error {
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

func champSelectSessionEvent(eventType string, phase string) ports.LCUEvent {
	return ports.LCUEvent{
		EventType: eventType,
		URI:       "/lol-champ-select/v1/session",
		Data:      json.RawMessage(`{"timer":{"phase":"` + phase + `"}}`),
	}
}

func sendWatchEvent(t *testing.T, events chan<- ports.LCUEvent, event ports.LCUEvent) {
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
