package lolautobuild

import (
	"context"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/controlado/lol-autobuild/internal/position"
	"github.com/controlado/lol-autobuild/internal/recommend"
)

func TestWatchRunsStartupSyncAndStopsGracefully(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID: 240,
			Position:   position.Mid,
			QueueID:    420,
		},
		watchEventsFn: func(ctx context.Context, out chan<- ports.LCUEvent) error {
			_ = out
			<-ctx.Done()
			return nil
		},
	}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 2)
	errCh := make(chan error, 1)

	go func() {
		errCh <- svc.Watch(ctx, WatchRequest{
			ApplyItems:  true,
			ApplyRunes:  true,
			ApplySpells: true,
			DryRun:      true,
			Debounce:    20 * time.Millisecond,
			OnCycle: func(cycle WatchCycle) {
				cycleCh <- cycle
				if cycle.Trigger == WatchTriggerStartup {
					cancel()
				}
			},
		})
	}()

	var startup WatchCycle
	select {
	case startup = <-cycleCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for startup cycle")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch exit")
	}

	if startup.Trigger != WatchTriggerStartup {
		t.Fatalf("expected startup trigger, got %q", startup.Trigger)
	}
	if startup.Result == nil {
		t.Fatal("expected startup sync result")
	}
	if lcu.detectCalls != 1 {
		t.Fatalf("expected one sync cycle, got detectCalls=%d", lcu.detectCalls)
	}
}

func TestWatchFiltersAndDebouncesEvents(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: ports.DetectedSelection{
			ChampionID: 240,
			Position:   position.Mid,
			QueueID:    420,
		},
		watchEventsFn: func(ctx context.Context, out chan<- ports.LCUEvent) error {
			select {
			case out <- ports.LCUEvent{EventType: "Create", URI: "/lol-champ-select/v1/session"}:
			case <-ctx.Done():
				return nil
			}
			select {
			case out <- ports.LCUEvent{EventType: "Update", URI: "/lol-champ-select/v1/session"}:
			case <-ctx.Done():
				return nil
			}
			select {
			case out <- ports.LCUEvent{EventType: "Delete", URI: "/lol-champ-select/v1/session"}:
			case <-ctx.Done():
				return nil
			}
			select {
			case out <- ports.LCUEvent{EventType: "Create", URI: "/lol-champ-select/v1/grid"}:
			case <-ctx.Done():
				return nil
			}

			<-ctx.Done()
			return nil
		},
	}

	svc, err := NewService(ServiceDeps{
		Coachless:   coachless,
		Tokens:      tokenProviderStub{token: "t"},
		LCU:         lcu,
		Recommender: recommend.NewEngine(),
		Policy:      RecommendationPolicy{MinOccurrence: 100, TopItems: 6, TopSpells: 2},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cycleCh := make(chan WatchCycle, 8)
	errCh := make(chan error, 1)

	go func() {
		errCh <- svc.Watch(ctx, WatchRequest{
			ApplyItems:  true,
			ApplyRunes:  true,
			ApplySpells: true,
			DryRun:      true,
			Debounce:    40 * time.Millisecond,
			OnCycle: func(cycle WatchCycle) {
				cycleCh <- cycle
				if cycle.Trigger == WatchTriggerEvent {
					cancel()
				}
			},
		})
	}()

	cycles := make([]WatchCycle, 0, 3)
	for len(cycles) < 2 {
		select {
		case cycle := <-cycleCh:
			cycles = append(cycles, cycle)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for watch cycles, got %d", len(cycles))
		}
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch exit")
	}

	if cycles[0].Trigger != WatchTriggerStartup {
		t.Fatalf("expected first cycle startup, got %q", cycles[0].Trigger)
	}
	if cycles[1].Trigger != WatchTriggerEvent {
		t.Fatalf("expected second cycle event, got %q", cycles[1].Trigger)
	}
	if cycles[1].EventURI != "/lol-champ-select/v1/session" {
		t.Fatalf("expected event uri /lol-champ-select/v1/session, got %q", cycles[1].EventURI)
	}
	if cycles[1].EventType != "Update" {
		t.Fatalf("expected last debounced event type Update, got %q", cycles[1].EventType)
	}
	if lcu.detectCalls != 2 {
		t.Fatalf("expected startup + 1 debounced cycle, got detectCalls=%d", lcu.detectCalls)
	}
}
