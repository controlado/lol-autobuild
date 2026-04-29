package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
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
		{name: "locale selector", want: `id="localeSelect"`},
		{name: "translation cache", want: `const translations = Object.create(null);`},
		{name: "english locale asset", want: `"/i18n/en.json"`},
		{name: "brazilian portuguese locale asset", want: `"pt-BR": "/i18n/pt-BR.json"`},
		{name: "locale loader", want: `async function loadLocale(locale)`},
		{name: "locale fallback loader", want: `async function ensureLocale(locale)`},
		{name: "initialization waits for locale", want: `currentLocale = await ensureLocale(currentLocale);`},
		{name: "mode status is dynamic", want: `id="modeStatus">Simulation: no client changes</p>`},
		{name: "mode status renderer", want: `function renderModeStatus(settings)`},
		{name: "locale change keeps current form mode", want: `renderModeStatus(readSettings())`},
		{name: "dry run checkbox drives mode status", want: `ids.dryRun.addEventListener("change", () => renderModeStatus(readSettings()))`},
		{name: "update checking state", want: `let updateChecking = false;`},
		{name: "update status state", want: `let currentUpdateStatus = "idle";`},
		{name: "update button renderer", want: `function renderUpdateButton()`},
		{name: "update button keeps checking label", want: `updateChecking || currentUpdateStatus === "checking"`},
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

	notWanted := []string{
		`const translations = {`,
		`en: {`,
		`"pt-BR": {`,
	}
	for _, value := range notWanted {
		t.Run("without inline catalog "+value, func(t *testing.T) {
			if strings.Contains(body, value) {
				t.Fatalf("GET / body contains inline catalog marker %q", value)
			}
		})
	}
}

func TestI18NAssets(t *testing.T) {
	server, err := NewServer(Options{
		App:         stubApp{},
		OpenBrowser: func(string) error { return nil },
		Token:       "test-token",
		Out:         io.Discard,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantKey    string
		wantValue  string
	}{
		{
			name:       "english",
			target:     "/i18n/en.json",
			wantStatus: http.StatusOK,
			wantKey:    "action.check_updates",
			wantValue:  "Check updates",
		},
		{
			name:       "brazilian portuguese",
			target:     "/i18n/pt-BR.json",
			wantStatus: http.StatusOK,
			wantKey:    "action.check_updates",
			wantValue:  "Verificar atualizações",
		},
		{
			name:       "unknown locale",
			target:     "/i18n/es.json",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("GET %s status = %d, want %d", tt.target, rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body[tt.wantKey] != tt.wantValue {
				t.Fatalf("%s = %q, want %q", tt.wantKey, body[tt.wantKey], tt.wantValue)
			}
		})
	}
}

func TestI18NCatalogsHaveMatchingKeysAndPlaceholders(t *testing.T) {
	en := readI18NCatalog(t, "static/i18n/en.json")
	ptBR := readI18NCatalog(t, "static/i18n/pt-BR.json")

	for key, enValue := range en {
		ptValue, ok := ptBR[key]
		if !ok {
			t.Errorf("pt-BR missing key %q", key)
			continue
		}

		enPlaceholders := placeholders(enValue)
		ptPlaceholders := placeholders(ptValue)
		if strings.Join(enPlaceholders, ",") != strings.Join(ptPlaceholders, ",") {
			t.Errorf("placeholders for %q differ: en=%v pt-BR=%v", key, enPlaceholders, ptPlaceholders)
		}
	}

	for key := range ptBR {
		if _, ok := en[key]; !ok {
			t.Errorf("en missing key %q", key)
		}
	}
}

func readI18NCatalog(t *testing.T, path string) map[string]string {
	t.Helper()

	raw, err := staticFiles.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var catalog map[string]string
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	return catalog
}

var placeholderPattern = regexp.MustCompile(`\{[A-Za-z0-9_]+\}`)

func placeholders(value string) []string {
	matches := placeholderPattern.FindAllString(value, -1)
	sort.Strings(matches)
	return matches
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
