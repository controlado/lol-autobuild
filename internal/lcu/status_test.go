package lcu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionStatusOff(t *testing.T) {
	t.Parallel()

	status := (&Client{Enabled: false}).ConnectionStatus(context.Background())
	if status.State != ConnectionStateOff {
		t.Fatalf("expected off status, got %#v", status)
	}
}

func TestConnectionStatusNotConnected(t *testing.T) {
	t.Parallel()

	client := &Client{
		Enabled: true,
		discoverProcessConnections: func(context.Context) []connectionCandidate {
			return nil
		},
	}

	status := client.ConnectionStatus(context.Background())
	if status.State != ConnectionStateNotConnected {
		t.Fatalf("expected not connected status, got %#v", status)
	}
}

func TestConnectionStatusConnected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
	}{
		{name: "ok", status: http.StatusOK},
		{name: "not found still proves lcu responded", status: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/riotclient/ux-state" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != basicAuthHeader("secret") {
					t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
				}

				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, "{}")
			}))
			defer server.Close()

			client := &Client{
				Enabled: true,
				discoverProcessConnections: func(context.Context) []connectionCandidate {
					return []connectionCandidate{
						staticCandidate("test", connectionInfo{
							Port:     mustServerPort(t, server.URL),
							Password: "secret",
							Protocol: "http",
						}),
					}
				},
			}

			status := client.ConnectionStatus(context.Background())
			if status.State != ConnectionStateConnected {
				t.Fatalf("expected connected status, got %#v", status)
			}
			if status.Source != "test" {
				t.Fatalf("expected source test, got %q", status.Source)
			}
		})
	}
}

func TestConnectionStatusProbeFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{
		Enabled: true,
		discoverProcessConnections: func(context.Context) []connectionCandidate {
			return []connectionCandidate{
				staticCandidate("test", connectionInfo{
					Port:     mustServerPort(t, server.URL),
					Password: "secret",
					Protocol: "http",
				}),
			}
		},
	}

	status := client.ConnectionStatus(context.Background())
	if status.State != ConnectionStateNotConnected {
		t.Fatalf("expected not connected status, got %#v", status)
	}
}

func TestConnectionStatusProbeRefusedIsNotLockfileMissing(t *testing.T) {
	t.Parallel()

	client := &Client{
		Enabled: true,
		discoverProcessConnections: func(context.Context) []connectionCandidate {
			return []connectionCandidate{
				staticCandidate("process:1234", connectionInfo{
					Port:     mustClosedTCPPort(t),
					Password: "secret",
					Protocol: "http",
				}),
			}
		},
	}

	status := client.ConnectionStatus(context.Background())
	if status.State != ConnectionStateNotConnected {
		t.Fatalf("expected not connected status, got %#v", status)
	}
	if !errors.Is(status.Err, ErrLCUNotReachable) {
		t.Fatalf("status error = %v, want %v", status.Err, ErrLCUNotReachable)
	}
	if errors.Is(status.Err, ErrLockfileNotFound) {
		t.Fatalf("status error = %v, should not wrap lockfile missing", status.Err)
	}
}
