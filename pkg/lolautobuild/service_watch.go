package lolautobuild

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

const defaultWatchDebounce = 500 * time.Millisecond

func (s *syncService) Watch(ctx context.Context, req WatchRequest) error {
	debounce := req.Debounce
	if debounce <= 0 {
		debounce = defaultWatchDebounce
	}

	_ = s.runWatchCycle(ctx, req, WatchTriggerStartup, nil)
	if ctx.Err() != nil {
		return nil
	}

	events := make(chan ports.LCUEvent, 64)
	watchErrCh := make(chan error, 1)

	go func() {
		watchErrCh <- s.deps.LCU.WatchEvents(ctx, events)
	}()

	var (
		timer        *time.Timer
		timerCh      <-chan time.Time
		hasPending   bool
		pendingEvent ports.LCUEvent
	)
	defer stopWatchTimer(timer)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-watchErrCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("watch lcu events: %w", err)
		case event := <-events:
			if !isChampSelectSessionEvent(event) {
				continue
			}

			hasPending = true
			pendingEvent = event
			timer, timerCh = resetWatchTimer(timer, debounce)
		case <-timerCh:
			timerCh = nil
			if !hasPending {
				continue
			}

			hasPending = false
			_ = s.runWatchCycle(ctx, req, WatchTriggerEvent, &pendingEvent)
			if ctx.Err() != nil {
				return nil
			}
		}
	}
}

func (s *syncService) runWatchCycle(ctx context.Context, req WatchRequest, trigger WatchTrigger, event *ports.LCUEvent) error {
	result, err := s.Sync(ctx, req.syncRequest())

	cycle := WatchCycle{
		Trigger: trigger,
		Err:     err,
	}
	if event != nil {
		cycle.EventType = event.EventType
		cycle.EventURI = event.URI
	}
	if err == nil {
		resultCopy := result
		cycle.Result = &resultCopy
	}

	if req.OnCycle != nil {
		req.OnCycle(cycle)
	}

	return err
}

func (r WatchRequest) syncRequest() SyncRequest {
	return SyncRequest{
		Patch:       r.Patch,
		ApplyItems:  r.ApplyItems,
		ApplyRunes:  r.ApplyRunes,
		ApplySpells: r.ApplySpells,
		DryRun:      r.DryRun,
	}
}

func isChampSelectSessionEvent(event ports.LCUEvent) bool {
	uri := strings.TrimSpace(event.URI)
	if uri != "/lol-champ-select/v1/session" {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(event.EventType)) {
	case "create", "update":
		return true
	default:
		return false
	}
}

func resetWatchTimer(timer *time.Timer, delay time.Duration) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(delay)
		return timer, timer.C
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
	return timer, timer.C
}

func stopWatchTimer(timer *time.Timer) {
	if timer == nil {
		return
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
