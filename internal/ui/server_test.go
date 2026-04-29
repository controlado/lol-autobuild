package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/app"
)

type stubUIApp struct{}

func (stubUIApp) State(context.Context) (s app.State)                                  { return }
func (stubUIApp) SaveSettings(context.Context, app.Settings) (s app.State, msg string) { return }
func (stubUIApp) RunSync(context.Context) (s app.State, msg string)                    { return }
func (stubUIApp) StartWatcher(context.Context) (s app.State, msg string)               { return }
func (stubUIApp) StopWatcher(context.Context) (s app.State)                            { return }
func (stubUIApp) CheckUpdates(context.Context) (s app.State, msg string)               { return }

func TestIndexRendersSyncSuboptionsStructure(t *testing.T) {
	server, err := NewServer(Options{
		App:         stubUIApp{},
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
