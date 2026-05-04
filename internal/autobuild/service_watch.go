package autobuild

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
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

	var (
		watchErrCh = make(chan error, 1)
		eventsCh   = make(chan domain.LCUEvent, 64)
		noticesCh  chan domain.LCUWatchNotice
	)

	if req.OnNotice != nil {
		noticesCh = make(chan domain.LCUWatchNotice, 64)
	}

	go func() {
		watchErrCh <- s.deps.LCU.WatchEventsWithNotices(ctx, eventsCh, noticesCh)
	}()

	var (
		pending = watchPendingCycle{}
		gate    = newWatchSessionGate()
	)
	defer pending.stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-watchErrCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("watch lcu events: %w", err)
		case notice := <-noticesCh:
			req.OnNotice(watchNoticeFromLCU(notice))
		case event := <-eventsCh:
			eventType, isSessionEvent, isFinalizationEvent := classifyChampSelectSessionEvent(event)
			if isSessionEvent && (eventType == "delete" || (eventType == "create" && !isFinalizationEvent)) {
				gate.reset(event)
				pending.clear()
				continue
			}

			if !isFinalizationEvent {
				continue
			}

			sessionKey, promotedFrom := gate.trackFinalizationSession(event, eventType)
			pending.promoteSessionKey(promotedFrom, sessionKey)
			if gate.isPendingOrSynced(sessionKey) {
				continue
			}

			gate.markPending(sessionKey)
			pending.schedule(event, sessionKey, debounce)
		case <-pending.timerC():
			event, sessionKey, ok := pending.consume()
			if !ok {
				continue
			}
			gate.markSynced(sessionKey)

			_ = s.runWatchCycle(ctx, req, watchTriggerForEvent(event), &event)
			if ctx.Err() != nil {
				return nil
			}
		}
	}
}

func (s *syncService) runWatchCycle(ctx context.Context, req WatchRequest, trigger WatchTrigger, event *domain.LCUEvent) error {
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

type watchPendingCycle struct {
	timer      *time.Timer
	timerCh    <-chan time.Time
	event      domain.LCUEvent
	sessionKey string
	scheduled  bool
}

func (p *watchPendingCycle) timerC() <-chan time.Time {
	return p.timerCh
}

func (p *watchPendingCycle) schedule(event domain.LCUEvent, sessionKey string, debounce time.Duration) {
	p.event = event
	p.sessionKey = sessionKey
	p.scheduled = true
	p.timer, p.timerCh = resetWatchTimer(p.timer, debounce)
}

func (p *watchPendingCycle) promoteSessionKey(from string, to string) {
	if from != "" && p.sessionKey == from {
		p.sessionKey = to
	}
}

func (p *watchPendingCycle) consume() (domain.LCUEvent, string, bool) {
	p.timerCh = nil
	if !p.scheduled {
		return domain.LCUEvent{}, "", false
	}

	event := p.event
	sessionKey := p.sessionKey
	p.event = domain.LCUEvent{}
	p.sessionKey = ""
	p.scheduled = false
	return event, sessionKey, true
}

func (p *watchPendingCycle) clear() {
	p.event = domain.LCUEvent{}
	p.sessionKey = ""
	p.scheduled = false
	p.stop()
}

func (p *watchPendingCycle) stop() {
	stopWatchTimer(p.timer)
	p.timer = nil
	p.timerCh = nil
}

type watchSessionGate struct {
	synced             map[string]struct{}
	pending            map[string]struct{}
	activeSessionKey   string
	activeConnectionID int
	nextAnonymousID    int
}

func newWatchSessionGate() *watchSessionGate {
	return &watchSessionGate{
		synced:  make(map[string]struct{}),
		pending: make(map[string]struct{}),
	}
}

func (g *watchSessionGate) reset(event domain.LCUEvent) {
	g.pending = make(map[string]struct{})

	if event.ConnectionID > 0 && event.ConnectionID != g.activeConnectionID {
		g.activeConnectionID = event.ConnectionID
	}

	g.activeSessionKey = ""
}

func (g *watchSessionGate) isPendingOrSynced(sessionKey string) bool {
	if sessionKey == "" {
		return false
	}

	if _, ok := g.synced[sessionKey]; ok {
		return true
	}

	_, ok := g.pending[sessionKey]
	return ok
}

func (g *watchSessionGate) markPending(sessionKey string) {
	if sessionKey == "" {
		return
	}

	g.pending[sessionKey] = struct{}{}
}

func (g *watchSessionGate) markSynced(sessionKey string) {
	if sessionKey == "" {
		return
	}

	delete(g.pending, sessionKey)
	g.synced[sessionKey] = struct{}{}
}

func (g *watchSessionGate) trackFinalizationSession(event domain.LCUEvent, eventType string) (sessionKey string, promotedFrom string) {
	connectionChanged := event.ConnectionID > 0 && event.ConnectionID != g.activeConnectionID
	if connectionChanged {
		g.activeConnectionID = event.ConnectionID
	}

	if gameID := gameIDFromEvent(event); gameID != "" {
		gameKey := "game:" + gameID
		return gameKey, g.promoteActiveSession(gameKey)
	}

	if connectionChanged {
		g.activeSessionKey = ""
	}

	if eventType == "create" || g.activeSessionKey == "" {
		g.nextAnonymousID++
		g.activeSessionKey = fmt.Sprintf("anonymous:%d", g.nextAnonymousID)
	}

	return g.activeSessionKey, ""
}

func (g *watchSessionGate) promoteActiveSession(gameKey string) string {
	if g.activeSessionKey == "" || g.activeSessionKey == gameKey {
		g.activeSessionKey = gameKey
		return ""
	}

	if !strings.HasPrefix(g.activeSessionKey, "anonymous:") {
		g.activeSessionKey = gameKey
		return ""
	}

	previousKey := g.activeSessionKey
	if _, ok := g.pending[g.activeSessionKey]; ok {
		delete(g.pending, g.activeSessionKey)
		g.pending[gameKey] = struct{}{}
	}
	if _, ok := g.synced[g.activeSessionKey]; ok {
		delete(g.synced, g.activeSessionKey)
		g.synced[gameKey] = struct{}{}
	}

	g.activeSessionKey = gameKey
	return previousKey
}

func watchTriggerForEvent(event domain.LCUEvent) WatchTrigger {
	if event.Source == domain.LCUEventSourceSnapshot {
		return WatchTriggerSnapshot
	}

	return WatchTriggerEvent
}

func watchNoticeFromLCU(notice domain.LCUWatchNotice) WatchNotice {
	return WatchNotice{
		Kind:         WatchNoticeKind(notice.Kind),
		Message:      notice.Message,
		Err:          notice.Err,
		Source:       notice.Source,
		URI:          notice.URI,
		Phase:        notice.Phase,
		ConnectionID: notice.ConnectionID,
	}
}

func (r WatchRequest) syncRequest() SyncRequest {
	return SyncRequest{
		Patch:              r.Patch,
		PatchAdditionsMode: r.PatchAdditionsMode,
		PatchAdditions:     r.PatchAdditions,
		LeagueTierPreset:   r.LeagueTierPreset,
		ApplyItems:         r.ApplyItems,
		ApplyRunes:         r.ApplyRunes,
		ApplySpells:        r.ApplySpells,
		KeepFlash:          r.KeepFlash,
		DryRun:             r.DryRun,
	}
}

func classifyChampSelectSessionEvent(event domain.LCUEvent) (eventType string, isSessionEvent bool, isFinalizationEvent bool) {
	eventType = strings.ToLower(strings.TrimSpace(event.EventType))
	if strings.TrimSpace(event.URI) != champSelectSessionURI {
		return eventType, false, false
	}

	switch eventType {
	case "create", "update", "snapshot":
	default:
		return eventType, true, false
	}

	return eventType, true, strings.EqualFold(strings.TrimSpace(event.ChampSelectPhase), "FINALIZATION")
}

func gameIDFromEvent(event domain.LCUEvent) string {
	value := strings.TrimSpace(event.GameID)
	if value == "" || value == "0" {
		return ""
	}
	return value
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
