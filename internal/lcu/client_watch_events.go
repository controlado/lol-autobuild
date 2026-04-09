package lcu

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/gorilla/websocket"
)

var ErrWatchEventStreamFailed = errors.New("watch event stream failed")
var ErrWatchEventsChannelRequired = errors.New("watch events channel is required")

const lcuEventTopic = "OnJsonApiEvent"

type lcuEventEnvelope struct {
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

func (c *Client) dialEventStream(ctx context.Context) (*websocket.Conn, error) {
	var lastErr error
	seenExisting := false

	for _, lockfilePath := range c.lockfileCandidates() {
		stat, err := os.Stat(lockfilePath)
		if err != nil || stat.IsDir() {
			continue
		}
		seenExisting = true

		info, err := c.readLockfile(lockfilePath)
		if err != nil {
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		conn, err := dialLCUWebsocket(ctx, info)
		if err != nil {
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		if err := conn.WriteJSON([]any{5, lcuEventTopic}); err != nil {
			_ = conn.Close()
			lastErr = fmt.Errorf("lockfile %q: subscribe %q: %w", lockfilePath, lcuEventTopic, err)
			continue
		}

		return conn, nil
	}

	if !seenExisting {
		return nil, ErrLockfileNotFound
	}

	return nil, withLastCandidateError(ErrWatchEventStreamFailed, lastErr)
}

func dialLCUWebsocket(ctx context.Context, info lockfileInfo) (*websocket.Conn, error) {
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
	headers.Set("Authorization", lcuBasicAuthHeader(info.Password))

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

		event, ok, err := decodeLCUEvent(payload)
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

func decodeLCUEvent(payload []byte) (ports.LCUEvent, bool, error) {
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
	if topic != lcuEventTopic {
		return ports.LCUEvent{}, false, nil
	}

	var envelope lcuEventEnvelope
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
