package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/app"
)

type stubApp struct {
	state app.State
}

func (sa stubApp) State(context.Context) (s app.State) {
	return sa.state
}
func (sa stubApp) SaveSettings(context.Context, app.Settings) (s app.State, msg app.UserMessage) {
	return
}
func (sa stubApp) RunSync(context.Context) (s app.State, msg app.UserMessage) {
	return
}
func (sa stubApp) StartWatcher(context.Context) (s app.State, msg app.UserMessage) {
	return
}
func (sa stubApp) StopWatcher(context.Context) (s app.State) {
	return
}
func (sa stubApp) CheckUpdates(context.Context) (s app.State, msg app.UserMessage) {
	return
}

func TestIndexRendersSyncSuboptionsStructure(t *testing.T) {
	server, err := NewServer(Options{
		App:         stubApp{},
		OpenBrowser: func(string) error { return nil },
		Token:       "test-token",
		Out:         io.Discard,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	tests := []struct {
		name string
		want string
	}{
		{name: "dropdown button id", want: `id="spellsOptionsButton"`},
		{name: "dropdown button class", want: `class="sync-dropdown-toggle"`},
		{name: "dropdown button label", want: `aria-label="Toggle summoner spell options"`},
		{name: "dropdown button target", want: `aria-controls="spellsSuboptions"`},
		{name: "dropdown starts collapsed", want: `aria-expanded="false"`},
		{name: "suboptions container id", want: `id="spellsSuboptions"`},
		{name: "keep flash checkbox id", want: `id="keepFlash"`},
		{name: "state sync function", want: `function syncSpellsSuboptions()`},
		{name: "base option drives suboptions", want: `ids.applySpells.addEventListener("change", syncSpellsSuboptions)`},
		{name: "settings contract preserved", want: `keep_flash: ids.keepFlash.checked`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(body, tt.want) {
				t.Fatalf("GET / body does not contain %q", tt.want)
			}
		})
	}

}

func TestAPIErrorIncludesCode(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		wantStatus int
		wantError  string
		wantCode   string
	}{
		{
			name:       "invalid token",
			method:     http.MethodGet,
			target:     "/api/state",
			wantStatus: http.StatusUnauthorized,
			wantError:  "Invalid UI token.",
			wantCode:   "ui.invalid_token",
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			target:     "/api/state?token=test-token",
			wantStatus: http.StatusMethodNotAllowed,
			wantError:  "Method is not allowed.",
			wantCode:   "ui.method_not_allowed",
		},
		{
			name:       "invalid settings",
			method:     http.MethodPost,
			target:     "/api/config?token=test-token",
			body:       `{"unknown":true}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "Settings are invalid.",
			wantCode:   "ui.invalid_settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(Options{
				App:         stubApp{},
				OpenBrowser: func(string) error { return nil },
				Token:       "test-token",
				Out:         io.Discard,
			})
			if err != nil {
				t.Fatalf("NewServer() error = %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.target, rec.Code, tt.wantStatus)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tt.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tt.wantError)
			}
			if body["error_code"] != tt.wantCode {
				t.Fatalf("error_code = %q, want %q", body["error_code"], tt.wantCode)
			}
		})
	}
}
