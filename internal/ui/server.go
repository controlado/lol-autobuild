package ui

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/app"
)

//go:embed static/index.html
var staticFiles embed.FS

type App interface {
	State(ctx context.Context) app.State
	SaveSettings(ctx context.Context, settings app.Settings) (app.State, string)
	RunSync(ctx context.Context) (app.State, string)
	StartWatcher(ctx context.Context) (app.State, string)
	StopWatcher(ctx context.Context) app.State
}

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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
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
	fmt.Fprintf(s.out, "Open %s\n", url)
	if err := s.openBrowser(url); err != nil {
		fmt.Fprintf(s.out, "Browser did not open. Use %s\n", url)
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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/config", s.handleSaveConfig)
	mux.HandleFunc("/api/sync", s.handleRunSync)
	mux.HandleFunc("/api/watch/start", s.handleStartWatch)
	mux.HandleFunc("/api/watch/stop", s.handleStopWatch)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Query().Get("token") != s.token {
			writeError(w, http.StatusUnauthorized, "Invalid UI token.")
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
		writeError(w, http.StatusInternalServerError, "UI file is missing.")
		return
	}

	page := strings.ReplaceAll(string(raw), "__API_TOKEN__", s.token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, page)
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
		writeError(w, http.StatusBadRequest, "Settings are invalid.")
		return
	}

	state, errMessage := s.app.SaveSettings(r.Context(), settings)
	if errMessage != "" {
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
	if errMessage != "" {
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
	if errMessage != "" {
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
	writeError(w, http.StatusMethodNotAllowed, "Method is not allowed.")
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate ui token: %w", err)
	}

	return hex.EncodeToString(raw), nil
}
