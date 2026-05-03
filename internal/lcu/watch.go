package lcu

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/gorilla/websocket"
)

const (
	eventTopic            = "OnJsonApiEvent"
	champSelectSessionURI = "/lol-champ-select/v1/session"
	snapshotEventType     = "Snapshot"
)

var (
	ErrWatchEventStreamFailed   = errors.New("watch event stream failed")
	ErrLCUEventsChannelRequired = errors.New("watch events channel is required")
)

type eventEnvelope struct {
	EventType string          `json:"eventType"`
	URI       string          `json:"uri"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) WatchEventsWithNotices(ctx context.Context, out chan<- ports.LCUEvent, notices chan<- ports.LCUWatchNotice) error {
	if !c.Enabled {
		return ErrNotConfigured
	}
	if out == nil {
		return ErrLCUEventsChannelRequired
	}

	var (
		reconnectDelay = c.watchReconnectDelay()
		connectionID   int
	)

	for {
		if ctx.Err() != nil {
			return nil
		}

		conn, info, source, err := c.dialEventStream(ctx)
		if err != nil {
			if !emitWatchNotice(ctx, notices, ports.LCUWatchNotice{
				Kind:    ports.LCUWatchNoticeReconnecting,
				Message: "LCU websocket is unavailable; waiting before reconnect.",
				Err:     err,
			}) {
				return nil
			}
			if !waitReconnect(ctx, reconnectDelay) {
				return nil
			}
			continue
		}
		connectionID++

		if !emitWatchNotice(ctx, notices, ports.LCUWatchNotice{
			Kind:         ports.LCUWatchNoticeConnected,
			Message:      "LCU websocket connected.",
			Source:       source,
			ConnectionID: connectionID,
		}) {
			_ = conn.Close()
			return nil
		}

		if ok := c.emitSessionSnapshot(ctx, info, source, connectionID, out, notices); !ok {
			_ = conn.Close()
			return nil
		}

		err = c.consumeEventStream(ctx, conn, out, connectionID)
		_ = conn.Close()

		if ctx.Err() != nil {
			return nil
		}

		if err == nil {
			continue
		}

		if !emitWatchNotice(ctx, notices, ports.LCUWatchNotice{
			Kind:         ports.LCUWatchNoticeReconnecting,
			Message:      "LCU websocket disconnected; waiting before reconnect.",
			Err:          err,
			Source:       source,
			ConnectionID: connectionID,
		}) {
			return nil
		}
		if !waitReconnect(ctx, reconnectDelay) {
			return nil
		}
	}
}

func (c *Client) watchReconnectDelay() time.Duration {
	if c.WatchReconnectDelay <= 0 {
		return time.Second
	}

	return c.WatchReconnectDelay
}

func (c *Client) dialEventStream(ctx context.Context) (conn *websocket.Conn, info connectionInfo, source string, err error) {
	var (
		attempt          = newConnectionAttempt()
		selectedInfo     connectionInfo
		candidateHandler = func(candidateInfo connectionInfo, candidateLabel string) (shouldTerminate bool) {
			conn, err = dialWebsocket(ctx, candidateInfo)
			if err != nil {
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			var message = []any{5, eventTopic}
			if err := conn.WriteJSON(message); err != nil {
				_ = conn.Close()
				err = fmt.Errorf("subscribe %q: %w", eventTopic, err)
				attempt.observe(candidateLabel, nil, err)
				return false
			}

			source = candidateLabel
			selectedInfo = candidateInfo
			return true
		}
	)

	if success, err := c.forEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return nil, connectionInfo{}, "", err
	} else if success {
		return conn, selectedInfo, source, nil
	}

	return nil, connectionInfo{}, "", attempt.finish(ErrWatchEventStreamFailed)
}

func dialWebsocket(ctx context.Context, info connectionInfo) (*websocket.Conn, error) {
	scheme := "ws"
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	if info.Protocol == "https" {
		scheme = "wss"
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	addr := fmt.Sprintf("%s://127.0.0.1:%d/", scheme, info.Port)
	headers := http.Header{}
	headers.Set("Authorization", basicAuthHeader(info.Password))

	conn, resp, err := dialer.DialContext(ctx, addr, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial websocket status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial websocket: %w", err)
	}

	return conn, nil
}

func (c *Client) consumeEventStream(ctx context.Context, conn *websocket.Conn, out chan<- ports.LCUEvent, connectionID int) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	defer close(done)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		event, ok, err := decodeEvent(payload)
		if err != nil || !ok {
			continue
		}
		event.Source = ports.LCUEventSourceStream
		event.ConnectionID = connectionID

		select {
		case out <- event:
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Client) emitSessionSnapshot(ctx context.Context, info connectionInfo, source string, connectionID int, out chan<- ports.LCUEvent, notices chan<- ports.LCUWatchNotice) bool {
	raw, err := c.fetchChampSelectSessionEventData(ctx, info)
	if err != nil {
		return emitWatchNotice(ctx, notices, ports.LCUWatchNotice{
			Kind:         ports.LCUWatchNoticeSnapshotWaiting,
			Message:      "Champ select snapshot is unavailable; waiting for websocket events.",
			Err:          err,
			Source:       source,
			URI:          champSelectSessionURI,
			ConnectionID: connectionID,
		})
	}

	phase := champSelectPhase(raw)
	notice := ports.LCUWatchNotice{
		Kind:         ports.LCUWatchNoticeSnapshotWaiting,
		Message:      "Champ select snapshot is not finalized; waiting for websocket events.",
		Source:       source,
		URI:          champSelectSessionURI,
		Phase:        phase,
		ConnectionID: connectionID,
	}
	if strings.EqualFold(strings.TrimSpace(phase), "FINALIZATION") {
		notice.Kind = ports.LCUWatchNoticeSnapshotFinalization
		notice.Message = "Champ select snapshot is finalized."
	}
	if !emitWatchNotice(ctx, notices, notice) {
		return false
	}

	if notice.Kind != ports.LCUWatchNoticeSnapshotFinalization {
		return true
	}

	select {
	case out <- ports.LCUEvent{
		EventType:    snapshotEventType,
		URI:          champSelectSessionURI,
		Data:         raw,
		Source:       ports.LCUEventSourceSnapshot,
		ConnectionID: connectionID,
	}:
		return true
	case <-ctx.Done():
		return false
	}
}

func champSelectPhase(raw json.RawMessage) string {
	var payload struct {
		Timer struct {
			Phase string `json:"phase"`
		} `json:"timer"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Timer.Phase)
}

func emitWatchNotice(ctx context.Context, notices chan<- ports.LCUWatchNotice, notice ports.LCUWatchNotice) bool {
	if notices == nil {
		return true
	}

	select {
	case notices <- notice:
		return true
	case <-ctx.Done():
		return false
	}
}

func decodeEvent(payload []byte) (ports.LCUEvent, bool, error) {
	var frame []json.RawMessage
	if err := json.Unmarshal(payload, &frame); err != nil {
		return ports.LCUEvent{}, false, err
	}

	if len(frame) < 3 {
		return ports.LCUEvent{}, false, nil
	}

	var topic string
	if err := json.Unmarshal(frame[1], &topic); err != nil {
		return ports.LCUEvent{}, false, err
	}
	if topic != eventTopic {
		return ports.LCUEvent{}, false, nil
	}

	var envelope eventEnvelope
	if err := json.Unmarshal(frame[2], &envelope); err != nil {
		return ports.LCUEvent{}, false, err
	}

	data := envelope.Data
	if len(data) == 0 {
		data = json.RawMessage("null")
	}

	return ports.LCUEvent{
		EventType: strings.TrimSpace(envelope.EventType),
		URI:       strings.TrimSpace(envelope.URI),
		Data:      append(json.RawMessage(nil), data...),
	}, true, nil
}

func waitReconnect(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
