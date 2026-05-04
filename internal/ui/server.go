package ui

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/app"
)

//go:embed static/index.html static/assets/*.css static/assets/*.js static/i18n/*.json
var staticFiles embed.FS

type App interface {
	State(ctx context.Context) app.ViewState
	SaveSettings(ctx context.Context, settings app.Settings) (app.ViewState, app.UserMessage)
	RunSync(ctx context.Context) (app.ViewState, app.UserMessage)
	StartWatcher(ctx context.Context) (app.ViewState, app.UserMessage)
	StopWatcher(ctx context.Context) app.ViewState
	CheckUpdates(ctx context.Context) (app.ViewState, app.UserMessage)
}

var (
	invalidUITokenMessage   = app.UserMessage{Code: "ui.invalid_token", Text: "Invalid UI token."}
	uiFileMissingMessage    = app.UserMessage{Code: "ui.file_missing", Text: "UI file is missing."}
	invalidSettingsMessage  = app.UserMessage{Code: "ui.invalid_settings", Text: "Settings are invalid."}
	methodNotAllowedMessage = app.UserMessage{Code: "ui.method_not_allowed", Text: "Method is not allowed."}
)

var i18nAssetPaths = map[string]string{
	"/i18n/en.json":    "static/i18n/en.json",
	"/i18n/pt-BR.json": "static/i18n/pt-BR.json",
}

type staticAsset struct {
	path        string
	contentType string
}

var staticAssetPaths = map[string]staticAsset{
	"/assets/app.js":     {path: "static/assets/app.js", contentType: "text/javascript; charset=utf-8"},
	"/assets/styles.css": {path: "static/assets/styles.css", contentType: "text/css; charset=utf-8"},
}

const (
	uiListenAddr         = "127.0.0.1:38473"
	uiFallbackListenAddr = "127.0.0.1:0"
)

type Server struct {
	app         App
	openBrowser BrowserOpener
	token       string
	out         io.Writer
}

func NewServer(opts Options) (*Server, error) {
	if opts.App == nil {
		return nil, fmt.Errorf("app cannot be nil")
	}
	if opts.OpenBrowser == nil {
		opts.OpenBrowser = OpenBrowser
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if strings.TrimSpace(opts.Token) == "" {
		token, err := randomToken()
		if err != nil {
			return nil, err
		}
		opts.Token = token
	}

	return &Server{
		app:         opts.App,
		openBrowser: opts.OpenBrowser,
		out:         opts.Out,
		token:       strings.TrimSpace(opts.Token),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	listener, usedPreferredAddr, err := listenUI(uiListenAddr, uiFallbackListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	if !usedPreferredAddr {
		_, _ = fmt.Fprintf(s.out, "Port %s is unavailable. Using %s\n", uiListenAddr, listener.Addr().String())
	}

	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(listener)
	}()

	url := fmt.Sprintf("http://%s/", listener.Addr().String())
	_, _ = fmt.Fprintf(s.out, "Open %s\n", url)
	if err := s.openBrowser(url); err != nil {
		_, _ = fmt.Fprintf(s.out, "Browser did not open. Use %s\n", url)
	}

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.app.StopWatcher(ctx)
			return err
		}
	}

	s.app.StopWatcher(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown ui server: %w", err)
	}

	return nil
}

func listenUI(preferredAddr, fallbackAddr string) (net.Listener, bool, error) {
	listener, err := net.Listen("tcp", preferredAddr)
	if err == nil {
		return listener, true, nil
	}

	fallbackListener, fallbackErr := net.Listen("tcp", fallbackAddr)
	if fallbackErr != nil {
		return nil, false, fmt.Errorf("%s: %w; fallback %s: %v", preferredAddr, err, fallbackAddr, fallbackErr)
	}

	return fallbackListener, false, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/assets/", s.handleStaticAsset)
	mux.HandleFunc("/i18n/", s.handleI18N)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/config", s.handleSaveConfig)
	mux.HandleFunc("/api/sync", s.handleRunSync)
	mux.HandleFunc("/api/watch/start", s.handleStartWatch)
	mux.HandleFunc("/api/watch/stop", s.handleStopWatch)
	mux.HandleFunc("/api/update/check", s.handleCheckUpdates)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Query().Get("token") != s.token {
			writeError(w, http.StatusUnauthorized, invalidUITokenMessage)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	raw, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, uiFileMissingMessage)
		return
	}

	page := strings.ReplaceAll(string(raw), "__API_TOKEN__", html.EscapeString(s.token))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, page)
}

func (s *Server) handleStaticAsset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	asset, ok := staticAssetPaths[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	raw, err := staticFiles.ReadFile(asset.path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, uiFileMissingMessage)
		return
	}

	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleI18N(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	assetPath, ok := i18nAssetPaths[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	raw, err := staticFiles.ReadFile(assetPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, uiFileMissingMessage)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, s.app.State(r.Context()))
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var settings app.Settings
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, invalidSettingsMessage)
		return
	}

	state, errMessage := s.app.SaveSettings(r.Context(), settings)
	if !errMessage.Empty() {
		writeError(w, http.StatusBadRequest, errMessage)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleRunSync(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	state, errMessage := s.app.RunSync(r.Context())
	if !errMessage.Empty() {
		writeError(w, http.StatusInternalServerError, errMessage)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleStartWatch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	state, errMessage := s.app.StartWatcher(r.Context())
	if !errMessage.Empty() {
		writeError(w, http.StatusInternalServerError, errMessage)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleStopWatch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	state := s.app.StopWatcher(r.Context())
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	state, errMessage := s.app.CheckUpdates(r.Context())
	if !errMessage.Empty() {
		writeError(w, http.StatusInternalServerError, errMessage)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func decodeJSON(r *http.Request, out any) error {
	defer func() { _ = r.Body.Close() }()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}

	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, methodNotAllowedMessage)
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message app.UserMessage) {
	payload := map[string]string{"error": message.Text}
	if message.Code != "" {
		payload["error_code"] = message.Code
	}
	writeJSON(w, status, payload)
}

func randomToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate ui token: %w", err)
	}

	return hex.EncodeToString(raw), nil
}
