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

func TestWatchEventsWithNoticesForwardsRawEventsAndStopsOnCancel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		notices chan ports.LCUWatchNotice
	}{
		{name: "nil notices"},
		{name: "buffered notices", notices: make(chan ports.LCUWatchNotice, 4)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			upgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool { return true },
			}

			expectedAuth := basicAuthHeader("secret")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != expectedAuth {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				if r.URL.Path == champSelectSessionURI {
					http.NotFound(w, r)
					return
				}

				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Errorf("upgrade websocket: %v", err)
					return
				}
				defer func() { _ = conn.Close() }()

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
				if len(sub) < 2 || json.Unmarshal(sub[1], &topic) != nil || topic != eventTopic {
					t.Errorf("unexpected subscription payload: %s", string(subRaw))
					return
				}

				if err := conn.WriteJSON([]any{
					8,
					eventTopic,
					map[string]any{
						"eventType": "Create",
						"uri":       "/lol-champ-select/v1/session",
						"data":      map[string]any{"localPlayerCellId": 3},
					},
				}); err != nil {
					t.Errorf("write event frame: %v", err)
					return
				}
			}))
			defer server.Close()

			client := NewClient(true, "")
			client.WatchReconnectDelay = 10 * time.Millisecond
			client.discoverProcessConnections = func(context.Context) []connectionCandidate {
				return []connectionCandidate{
					staticCandidate("process:4321", connectionInfo{
						Port:     mustServerPort(t, server.URL),
						Password: "secret",
						Protocol: "http",
					}),
				}
			}

			events := make(chan ports.LCUEvent, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- client.WatchEventsWithNotices(ctx, events, tt.notices)
			}()

			select {
			case event := <-events:
				if event.EventType != "Create" {
					t.Fatalf("expected event type Create, got %q", event.EventType)
				}
				if event.URI != "/lol-champ-select/v1/session" {
					t.Fatalf("expected uri /lol-champ-select/v1/session, got %q", event.URI)
				}
				if event.Source != ports.LCUEventSourceStream {
					t.Fatalf("event.Source = %q, want %q", event.Source, ports.LCUEventSourceStream)
				}
				if event.ConnectionID != 1 {
					t.Fatalf("event.ConnectionID = %d, want 1", event.ConnectionID)
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
					t.Fatalf("WatchEventsWithNotices() error = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for WatchEventsWithNotices exit")
			}
		})
	}
}

func TestWatchEventsWithNoticesReconnectsAfterDisconnect(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == champSelectSessionURI {
			http.NotFound(w, r)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

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
			eventTopic,
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

	client := NewClient(true, "")
	client.WatchReconnectDelay = 20 * time.Millisecond
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{
			staticCandidate("process:5321", connectionInfo{
				Port:     mustServerPort(t, server.URL),
				Password: "secret",
				Protocol: "http",
			}),
		}
	}

	events := make(chan ports.LCUEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WatchEventsWithNotices(ctx, events, nil)
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
			t.Fatalf("WatchEventsWithNotices() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WatchEventsWithNotices exit")
	}
}

func TestWatchEventsWithNoticesEmitsConnectedAndFinalizationSnapshot(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == champSelectSessionURI {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gameId":9876,"timer":{"phase":"FINALIZATION"},"queueId":420,"localPlayerCellId":1,"myTeam":[]}`))
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read subscription frame: %v", err)
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(true, "")
	client.WatchReconnectDelay = 10 * time.Millisecond
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{
			staticCandidate("process:6321", connectionInfo{
				Port:     mustServerPort(t, server.URL),
				Password: "secret",
				Protocol: "http",
			}),
		}
	}

	events := make(chan ports.LCUEvent, 1)
	notices := make(chan ports.LCUWatchNotice, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WatchEventsWithNotices(ctx, events, notices)
	}()

	connected := waitForLCUNotice(t, notices, ports.LCUWatchNoticeConnected)
	if connected.ConnectionID != 1 {
		t.Fatalf("connected.ConnectionID = %d, want 1", connected.ConnectionID)
	}

	snapshot := waitForLCUNotice(t, notices, ports.LCUWatchNoticeSnapshotFinalization)
	if snapshot.Phase != "FINALIZATION" {
		t.Fatalf("snapshot.Phase = %q, want FINALIZATION", snapshot.Phase)
	}

	select {
	case event := <-events:
		if event.Source != ports.LCUEventSourceSnapshot {
			t.Fatalf("event.Source = %q, want %q", event.Source, ports.LCUEventSourceSnapshot)
		}
		if event.EventType != snapshotEventType {
			t.Fatalf("event.EventType = %q, want %q", event.EventType, snapshotEventType)
		}
		var payload struct {
			GameID int `json:"gameId"`
			Timer  struct {
				Phase string `json:"phase"`
			} `json:"timer"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("decode snapshot event data: %v", err)
		}
		if payload.GameID != 9876 || payload.Timer.Phase != "FINALIZATION" {
			t.Fatalf("snapshot payload = %+v, want gameId 9876 and FINALIZATION", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for snapshot event")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("WatchEventsWithNotices() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WatchEventsWithNotices exit")
	}
}

func TestWatchEventsWithNoticesReportsReconnectAndContinues(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == champSelectSessionURI {
			http.NotFound(w, r)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read subscription frame: %v", err)
			return
		}

		if connCount.Add(1) == 1 {
			return
		}

		if err := conn.WriteJSON([]any{
			8,
			eventTopic,
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

	client := NewClient(true, "")
	client.WatchReconnectDelay = 20 * time.Millisecond
	client.discoverProcessConnections = func(context.Context) []connectionCandidate {
		return []connectionCandidate{
			staticCandidate("process:7321", connectionInfo{
				Port:     mustServerPort(t, server.URL),
				Password: "secret",
				Protocol: "http",
			}),
		}
	}

	events := make(chan ports.LCUEvent, 1)
	notices := make(chan ports.LCUWatchNotice, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WatchEventsWithNotices(ctx, events, notices)
	}()

	_ = waitForLCUNotice(t, notices, ports.LCUWatchNoticeConnected)
	reconnecting := waitForLCUNotice(t, notices, ports.LCUWatchNoticeReconnecting)
	if reconnecting.Err == nil {
		t.Fatal("expected reconnecting notice error")
	}

	select {
	case event := <-events:
		if event.EventType != "Update" {
			t.Fatalf("expected Update event after reconnect, got %q", event.EventType)
		}
		if event.ConnectionID < 2 {
			t.Fatalf("event.ConnectionID = %d, want reconnect connection", event.ConnectionID)
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
			t.Fatalf("WatchEventsWithNotices() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WatchEventsWithNotices exit")
	}
}

func TestDecodeLCUEvent(t *testing.T) {
	t.Parallel()

	payload := []byte(`[8,"OnJsonApiEvent",{"eventType":"Create","uri":"/lol-champ-select/v1/session","data":{"foo":"bar"}}]`)

	event, ok, err := decodeEvent(payload)
	if err != nil {
		t.Fatalf("decodeEvent() error = %v", err)
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

func waitForLCUNotice(t *testing.T, notices <-chan ports.LCUWatchNotice, kind ports.LCUWatchNoticeKind) ports.LCUWatchNotice {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case notice := <-notices:
			if notice.Kind == kind {
				return notice
			}
		case <-deadline:
			t.Fatalf("timed out waiting for notice %q", kind)
			return ports.LCUWatchNotice{}
		}
	}
}
