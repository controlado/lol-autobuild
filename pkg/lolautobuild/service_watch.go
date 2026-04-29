package lolautobuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

const (
	defaultWatchDebounce  = 500 * time.Millisecond
	champSelectSessionURI = "/lol-champ-select/v1/session"
)

func (s *syncService) Watch(ctx context.Context, req WatchRequest) error {
	debounce := req.Debounce
	if debounce <= 0 {
		debounce = defaultWatchDebounce
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
		didFinalize  bool
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
			eventType, isSessionEvent, isFinalizationEvent := classifyChampSelectSessionEvent(event)
			if isSessionEvent && (eventType == "delete" || (eventType == "create" && !isFinalizationEvent)) {
				didFinalize = false
				hasPending = false
				pendingEvent = ports.LCUEvent{}

				stopWatchTimer(timer)
				timer = nil
				timerCh = nil
				continue
			}

			if !isFinalizationEvent || didFinalize {
				continue
			}

			didFinalize = true
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
		KeepFlash:   r.KeepFlash,
		DryRun:      r.DryRun,
	}
}

func classifyChampSelectSessionEvent(event ports.LCUEvent) (eventType string, isSessionEvent bool, isFinalizationEvent bool) {
	eventType = strings.ToLower(strings.TrimSpace(event.EventType))
	if strings.TrimSpace(event.URI) != champSelectSessionURI {
		return eventType, false, false
	}

	switch eventType {
	case "create", "update":
	default:
		return eventType, true, false
	}

	var payload struct {
		Timer struct {
			Phase string `json:"phase"`
		} `json:"timer"`
	}
	if len(event.Data) == 0 {
		return eventType, true, false
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return eventType, true, false
	}

	return eventType, true, strings.EqualFold(strings.TrimSpace(payload.Timer.Phase), "FINALIZATION")
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
