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

const eventTopic = "OnJsonApiEvent"

var (
	ErrWatchEventStreamFailed     = errors.New("watch event stream failed")
	ErrWatchEventsChannelRequired = errors.New("watch events channel is required")
)

type eventEnvelope struct {
	EventType string          `json:"eventType"`
	URI       string          `json:"uri"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) WatchEvents(ctx context.Context, out chan<- ports.LCUEvent) error {
	if !c.Enabled {
		return ErrNotConfigured
	}
	if out == nil {
		return ErrWatchEventsChannelRequired
	}

	reconnectDelay := c.watchReconnectDelay()

	for {
		if ctx.Err() != nil {
			return nil
		}

		conn, err := c.dialEventStream(ctx)
		if err != nil {
			if !waitReconnect(ctx, reconnectDelay) {
				return nil
			}
			continue
		}

		err = c.consumeEventStream(ctx, conn, out)
		_ = conn.Close()

		if ctx.Err() != nil {
			return nil
		}

		if err == nil {
			continue
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

func (c *Client) dialEventStream(ctx context.Context) (conn *websocket.Conn, err error) {
	var (
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (shouldTerminate bool) {
			conn, err = dialWebsocket(ctx, info)
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

			return true
		}
	)

	if success, err := c.ForEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return nil, err
	} else if success {
		return conn, nil
	}

	return nil, attempt.finish(ErrWatchEventStreamFailed)
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

func (c *Client) consumeEventStream(ctx context.Context, conn *websocket.Conn, out chan<- ports.LCUEvent) error {
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

		select {
		case out <- event:
		case <-ctx.Done():
			return nil
		}
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
