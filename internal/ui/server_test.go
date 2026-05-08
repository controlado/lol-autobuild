package ui

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/app"
)

type stubApp struct {
	state                    app.ViewState
	saveState                *app.ViewState
	saved                    app.Settings
	loginCoachlessAuthCalls  int
	logoutCoachlessAuthCalls int
}

func (sa *stubApp) State(context.Context) (s app.ViewState) {
	return sa.state
}
func (sa *stubApp) SaveSettings(_ context.Context, settings app.Settings) (s app.ViewState, msg app.UserMessage) {
	sa.saved = settings
	if sa.saveState != nil {
		s = *sa.saveState
		if s.Settings == (app.Settings{}) {
			s.Settings = settings
		}
		return s, app.UserMessage{}
	}
	s = app.ViewState{Settings: settings}
	return
}
func (sa *stubApp) LoginCoachlessAuth(context.Context) (s app.ViewState, msg app.UserMessage) {
	sa.loginCoachlessAuthCalls++
	return sa.state, app.UserMessage{}
}
func (sa *stubApp) LogoutCoachlessAuth(context.Context) (s app.ViewState, msg app.UserMessage) {
	sa.logoutCoachlessAuthCalls++
	return sa.state, app.UserMessage{}
}
func (sa *stubApp) RunSync(context.Context) (s app.ViewState, msg app.UserMessage) {
	return
}
func (sa *stubApp) StartWatcher(context.Context) (s app.ViewState, msg app.UserMessage) {
	return
}
func (sa *stubApp) StopWatcher(context.Context) (s app.ViewState) {
	return
}
func (sa *stubApp) CheckUpdates(context.Context) (s app.ViewState, msg app.UserMessage) {
	return
}

func TestListenUI(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) (preferredAddr string, cleanup func())
		wantPreferred bool
	}{
		{
			name: "uses preferred address",
			setup: func(t *testing.T) (string, func()) {
				listener := mustListenTCP(t, "127.0.0.1:0")
				addr := listener.Addr().String()
				if err := listener.Close(); err != nil {
					t.Fatalf("close probe listener: %v", err)
				}
				return addr, func() {}
			},
			wantPreferred: true,
		},
		{
			name: "falls back when preferred address is busy",
			setup: func(t *testing.T) (string, func()) {
				listener := mustListenTCP(t, "127.0.0.1:0")
				return listener.Addr().String(), func() {
					_ = listener.Close()
				}
			},
			wantPreferred: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preferredAddr, cleanup := tt.setup(t)
			defer cleanup()

			listener, usedPreferred, err := listenUI(preferredAddr, "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listenUI() error = %v", err)
			}
			go func() { _ = listener.Close() }()

			if usedPreferred != tt.wantPreferred {
				t.Fatalf("usedPreferred = %v, want %v", usedPreferred, tt.wantPreferred)
			}
			if usedPreferred && listener.Addr().String() != preferredAddr {
				t.Fatalf("listener address = %q, want %q", listener.Addr().String(), preferredAddr)
			}
			if !usedPreferred && listener.Addr().String() == preferredAddr {
				t.Fatalf("listener address = %q, want fallback address", listener.Addr().String())
			}
		})
	}
}

func mustListenTCP(t *testing.T, addr string) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen %s: %v", addr, err)
	}

	return listener
}

func TestIndexReferencesStaticAssetsAndEscapesToken(t *testing.T) {
	token := `test-token"<>&`
	server, err := NewServer(Options{
		App:         new(stubApp),
		OpenBrowser: func(string) error { return nil },
		Token:       token,
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
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`<link rel="stylesheet" href="/assets/styles.css">`,
		`<script src="/assets/app.js" defer></script>`,
		`<meta name="lol-autobuild-api-token" content="` + html.EscapeString(token) + `">`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("index body missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"<style>",
		"const token =",
		"__API_TOKEN__",
		token,
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("index body contains %q", unwanted)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	server, err := NewServer(Options{
		App:         new(stubApp),
		OpenBrowser: func(string) error { return nil },
		Token:       "test-token",
		Out:         io.Discard,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name            string
		method          string
		target          string
		wantStatus      int
		wantContentType string
		wantBody        string
		forbidBody      string
	}{
		{
			name:            "css",
			method:          http.MethodGet,
			target:          "/assets/styles.css",
			wantStatus:      http.StatusOK,
			wantContentType: "text/css",
			wantBody:        ":root",
		},
		{
			name:            "javascript",
			method:          http.MethodGet,
			target:          "/assets/app.js",
			wantStatus:      http.StatusOK,
			wantContentType: "text/javascript",
			wantBody:        "const tokenMeta",
			forbidBody:      "__API_TOKEN__",
		},
		{
			name:       "unknown asset",
			method:     http.MethodGet,
			target:     "/assets/missing.js",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			target:     "/assets/app.js",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.target, rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, tt.wantContentType) {
				t.Fatalf("Content-Type = %q, want %s", contentType, tt.wantContentType)
			}
			if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body missing %q", tt.wantBody)
			} else if tt.forbidBody != "" && strings.Contains(body, tt.forbidBody) {
				t.Fatalf("body contains %q", tt.forbidBody)
			}
		})
	}
}

func TestRuneSettingsAreEnabledInStaticUI(t *testing.T) {
	htmlRaw, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	jsRaw, err := staticFiles.ReadFile("static/assets/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}

	htmlBody := string(htmlRaw)
	jsBody := string(jsRaw)
	for _, unwanted := range []string{
		`id="applyRunes" disabled`,
		`data-i18n="state.coming_soon"`,
	} {
		if strings.Contains(htmlBody, unwanted) {
			t.Fatalf("rune checkbox still contains %q", unwanted)
		}
	}
	for _, want := range []string{
		"apply_runes: ids.applyRunes.checked",
		"ids.applyRunes.checked = Boolean(settings.apply_runes)",
		`ids.applyRunes.addEventListener("change", scheduleSave)`,
	} {
		if !strings.Contains(jsBody, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestI18NAssets(t *testing.T) {
	server, err := NewServer(Options{
		App:         new(stubApp),
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
				App:         new(stubApp),
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

func TestSaveConfigAcceptsAdvancedFilters(t *testing.T) {
	saveState := app.ViewState{Watcher: app.WatcherState{Running: true, ConfigStale: true}}
	recApp := &stubApp{saveState: &saveState}
	server, err := NewServer(Options{
		App:         recApp,
		OpenBrowser: func(string) error { return nil },
		Token:       "test-token",
		Out:         io.Discard,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	body := `{
		"patch":"16.8",
		"patch_additions_mode":"manual",
		"patch_additions":4,
		"league_tier_preset":"master_plus",
		"apply_items":true,
		"apply_runes":true,
		"apply_spells":true,
		"keep_flash":true,
		"dry_run":true,
		"lcu_enabled":true
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/config?token=test-token", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/config status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if recApp.saved.PatchAdditionsMode != "manual" || recApp.saved.PatchAdditions != 4 || recApp.saved.LeagueTierPreset != "master_plus" {
		t.Fatalf("saved advanced settings = %#v", recApp.saved)
	}
	if !recApp.saved.ApplyRunes {
		t.Fatalf("saved apply_runes = false, want true")
	}

	var state app.ViewState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if state.Settings.PatchAdditionsMode != "manual" || state.Settings.PatchAdditions != 4 || state.Settings.LeagueTierPreset != "master_plus" {
		t.Fatalf("response advanced settings = %#v", state.Settings)
	}
	if !state.Settings.ApplyRunes {
		t.Fatalf("response apply_runes = false, want true")
	}
	if !state.Watcher.ConfigStale {
		t.Fatal("response watcher.config_stale = false, want true")
	}
}

func TestCoachlessAuthEndpoints(t *testing.T) {
	tests := []struct {
		name            string
		target          string
		body            string
		wantLoginCalls  int
		wantLogoutCalls int
	}{
		{name: "login", target: "/api/coachless/auth/login?token=test-token", body: `{}`, wantLoginCalls: 1, wantLogoutCalls: 0},
		{name: "logout", target: "/api/coachless/auth/logout?token=test-token", body: `{}`, wantLoginCalls: 0, wantLogoutCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recApp := &stubApp{
				state: app.ViewState{
					CoachlessAuth: app.CoachlessAuthState{
						Status: app.CoachlessAuthStatusStored,
						Plan:   app.CoachlessAuthPlanPremium,
					},
				},
			}
			server, err := NewServer(Options{
				App:         recApp,
				OpenBrowser: func(string) error { return nil },
				Token:       "test-token",
				Out:         io.Discard,
			})
			if err != nil {
				t.Fatalf("NewServer() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("POST %s status = %d, want %d: %s", tt.target, rec.Code, http.StatusOK, rec.Body.String())
			}

			var state app.ViewState
			if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
				t.Fatalf("decode state: %v", err)
			}
			if state.CoachlessAuth.Status != app.CoachlessAuthStatusStored {
				t.Fatalf("state.CoachlessAuth = %+v", state.CoachlessAuth)
			}
			if tt.wantLoginCalls != recApp.loginCoachlessAuthCalls {
				t.Fatalf("loginCoachlessAuthCalls = %d, want %d", recApp.loginCoachlessAuthCalls, tt.wantLoginCalls)
			}
			if tt.wantLogoutCalls != recApp.logoutCoachlessAuthCalls {
				t.Fatalf("logoutCoachlessAuthCalls = %d, want %d", recApp.logoutCoachlessAuthCalls, tt.wantLogoutCalls)
			}
		})
	}
}

func TestCoachlessAuthEndpointErrors(t *testing.T) {
	server, err := NewServer(Options{
		App:         new(stubApp),
		OpenBrowser: func(string) error { return nil },
		Token:       "test-token",
		Out:         io.Discard,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "login requires ui token",
			method:     http.MethodPost,
			target:     "/api/coachless/auth/login",
			body:       `{}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "ui.invalid_token",
		},
		{
			name:       "logout is not GET",
			method:     http.MethodGet,
			target:     "/api/coachless/auth/logout?token=test-token",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "ui.method_not_allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("%s %s status = %d, want %d: %s", tt.method, tt.target, rec.Code, tt.wantStatus, rec.Body.String())
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error_code"] != tt.wantCode {
				t.Fatalf("error_code = %q, want %q", body["error_code"], tt.wantCode)
			}
		})
	}
}
