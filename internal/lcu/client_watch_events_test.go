package lcu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
	"github.com/gorilla/websocket"
)

func TestWatchEventsForwardsRawEventsAndStopsOnCancel(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	expectedAuth := lcuBasicAuthHeader("secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		_, subRaw, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read subscription frame: %v", err)
			return
		}
		var sub []json.RawMessage
		if err := json.Unmarshal(subRaw, &sub); err != nil {
			t.Errorf("unmarshal subscription frame: %v", err)
			return
		}

		var topic string
		if len(sub) < 2 || json.Unmarshal(sub[1], &topic) != nil || topic != lcuEventTopic {
			t.Errorf("unexpected subscription payload: %s", string(subRaw))
			return
		}

		if err := conn.WriteJSON([]any{
			8,
			lcuEventTopic,
			map[string]any{
				"eventType": "Create",
				"uri":       "/lol-champ-select/v1/session",
				"data": map[string]any{
					"localPlayerCellId": 3,
				},
			},
		}); err != nil {
			t.Errorf("write event frame: %v", err)
			return
		}
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	client := NewClient(true, "")
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate {
		return []clientConnectionCandidate{
			staticConnectionCandidate("process:4321", lockfileInfo{
				Port:     port,
				Password: "secret",
				Protocol: "http",
			}),
		}
	}
	client.WatchReconnectDelay = 10 * time.Millisecond

	events := make(chan ports.LCUEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WatchEvents(ctx, events)
	}()

	select {
	case event := <-events:
		if event.EventType != "Create" {
			t.Fatalf("expected event type Create, got %q", event.EventType)
		}
		if event.URI != "/lol-champ-select/v1/session" {
			t.Fatalf("expected uri /lol-champ-select/v1/session, got %q", event.URI)
		}
		if len(event.Data) == 0 {
			t.Fatal("expected non-empty raw event data")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarded event")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("WatchEvents() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WatchEvents exit")
	}
}

func TestWatchEventsReconnectsAfterDisconnect(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		_, _, err = conn.ReadMessage()
		if err != nil {
			t.Errorf("read subscription frame: %v", err)
			return
		}

		if connCount.Add(1) == 1 {
			return
		}

		if err := conn.WriteJSON([]any{
			8,
			lcuEventTopic,
			map[string]any{
				"eventType": "Update",
				"uri":       "/lol-champ-select/v1/session",
				"data":      map[string]any{"tick": 2},
			},
		}); err != nil {
			t.Errorf("write event frame: %v", err)
			return
		}
	}))
	defer server.Close()

	port := mustServerPort(t, server.URL)

	client := NewClient(true, "")
	client.discoverOpenClientConnections = func(context.Context) []clientConnectionCandidate {
		return []clientConnectionCandidate{
			staticConnectionCandidate("process:5321", lockfileInfo{
				Port:     port,
				Password: "secret",
				Protocol: "http",
			}),
		}
	}
	client.WatchReconnectDelay = 20 * time.Millisecond

	events := make(chan ports.LCUEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WatchEvents(ctx, events)
	}()

	select {
	case event := <-events:
		if event.EventType != "Update" {
			t.Fatalf("expected Update event after reconnect, got %q", event.EventType)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event after reconnect")
	}

	if connCount.Load() < 2 {
		t.Fatalf("expected at least 2 websocket connections, got %d", connCount.Load())
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("WatchEvents() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WatchEvents exit")
	}
}

func TestDecodeLCUEvent(t *testing.T) {
	t.Parallel()

	payload := []byte(`[8,"OnJsonApiEvent",{"eventType":"Create","uri":"/lol-champ-select/v1/session","data":{"foo":"bar"}}]`)

	event, ok, err := decodeLCUEvent(payload)
	if err != nil {
		t.Fatalf("decodeLCUEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("expected payload to be recognized as LCU event")
	}

	if event.EventType != "Create" {
		t.Fatalf("expected event type Create, got %q", event.EventType)
	}
	if event.URI != "/lol-champ-select/v1/session" {
		t.Fatalf("expected uri /lol-champ-select/v1/session, got %q", event.URI)
	}
	if string(event.Data) != `{"foo":"bar"}` {
		t.Fatalf("unexpected raw data: %s", string(event.Data))
	}
}
