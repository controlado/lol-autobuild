package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/config"
)

type stubAuthCommandSession struct {
	status       auth.CoachlessSessionState
	exportBundle string
	imported     string
	err          error

	loginCalls  int
	logoutCalls int
	exportCalls int
	importCalls int
}

func (s *stubAuthCommandSession) Status(context.Context) auth.CoachlessSessionState {
	return s.status
}

func (s *stubAuthCommandSession) Login(context.Context) error {
	s.loginCalls++
	return s.err
}

func (s *stubAuthCommandSession) Logout(context.Context) error {
	s.logoutCalls++
	return s.err
}

func (s *stubAuthCommandSession) Export(context.Context) (string, error) {
	s.exportCalls++
	if s.err != nil {
		return "", s.err
	}
	return s.exportBundle, nil
}

func (s *stubAuthCommandSession) Import(_ context.Context, raw string) error {
	s.importCalls++
	s.imported = raw
	return s.err
}

func TestAuthCommandVisibility(t *testing.T) {
	root := rootCmd()
	authRoot := findCommand(t, root.Commands(), "auth")
	if authRoot.Hidden {
		t.Fatal("auth command should be visible")
	}

	authCmd := authCmd()
	tests := []struct {
		name       string
		wantHidden bool
	}{
		{name: "status"},
		{name: "login"},
		{name: "logout"},
		{name: "export", wantHidden: true},
		{name: "import", wantHidden: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(t, authCmd.Commands(), tt.name)
			if cmd.Hidden != tt.wantHidden {
				t.Fatalf("%s Hidden = %v, want %v", tt.name, cmd.Hidden, tt.wantHidden)
			}
		})
	}
}

func TestAuthCommandRejectsMissingConfig(t *testing.T) {
	calls := 0
	cmd := newAuthCmd(func(config.Config) authCommandSession {
		calls++
		return &stubAuthCommandSession{}
	})

	_, _, err := executeAuthCommandWithConfig(t, cmd, t.TempDir()+"/missing.yaml", "", "status")
	if err == nil {
		t.Fatal("auth status error = nil, want missing config error")
	}
	if calls != 0 {
		t.Fatalf("session factory calls = %d, want 0", calls)
	}
}

func TestAuthCommandLoadsConfigBeforeSessionFactory(t *testing.T) {
	cfg := config.Defaults()
	cfg.Secrets.ServiceName = "test-service"
	configPath := writeTestConfigWith(t, cfg)
	session := &stubAuthCommandSession{}

	gotServiceName := ""
	cmd := newAuthCmd(func(cfg config.Config) authCommandSession {
		gotServiceName = cfg.Secrets.ServiceName
		return session
	})

	if _, _, err := executeAuthCommandWithConfig(t, cmd, configPath, "", "status"); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if gotServiceName != "test-service" {
		t.Fatalf("session factory service name = %q, want test-service", gotServiceName)
	}
}

func TestAuthSubcommandsRejectPositionalArgs(t *testing.T) {
	session := &stubAuthCommandSession{}

	for _, args := range [][]string{
		{"status", "extra"},
		{"login", "extra"},
		{"logout", "extra"},
		{"export", "extra"},
		{"import", "extra"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, _, err := executeAuthCommand(t, newTestAuthCommand(session), "", args...)
			if err == nil {
				t.Fatal("Execute() error = nil, want positional arg error")
			}
		})
	}
}

func TestAuthStatusPrintsSanitizedJSON(t *testing.T) {
	expiresAt := time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC)
	session := &stubAuthCommandSession{status: auth.CoachlessSessionState{
		Status:    auth.CoachlessSessionStatusStored,
		Plan:      auth.CoachlessPlanPremium,
		ExpiresAt: &expiresAt,
		Message:   "status only",
	}}

	stdout, _, err := executeAuthCommand(t, newTestAuthCommand(session), "", "status")
	if err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout, `"status": "stored"`) || !strings.Contains(stdout, `"plan": "premium"`) {
		t.Fatalf("auth status output = %s", stdout)
	}
	if strings.Contains(stdout, "access_token") || strings.Contains(stdout, "refresh_token") {
		t.Fatalf("auth status should not print tokens: %s", stdout)
	}
}

func TestAuthLoginAndLogoutCallSession(t *testing.T) {
	session := &stubAuthCommandSession{}
	cmd := newTestAuthCommand(session)

	if stdout, _, err := executeAuthCommand(t, cmd, "", "login"); err != nil {
		t.Fatalf("auth login error = %v", err)
	} else if !strings.Contains(stdout, "saved") {
		t.Fatalf("auth login stdout = %q", stdout)
	}
	if stdout, _, err := executeAuthCommand(t, cmd, "", "logout"); err != nil {
		t.Fatalf("auth logout error = %v", err)
	} else if !strings.Contains(stdout, "cleared") {
		t.Fatalf("auth logout stdout = %q", stdout)
	}

	if session.loginCalls != 1 || session.logoutCalls != 1 {
		t.Fatalf("auth calls = login:%d logout:%d", session.loginCalls, session.logoutCalls)
	}
}

func TestAuthExportWritesBundleToStdoutAndWarningToStderr(t *testing.T) {
	bundle := `{"access_token":"secret-access","refresh_token":"secret-refresh"}`
	session := &stubAuthCommandSession{exportBundle: bundle}

	stdout, stderr, err := executeAuthCommand(t, newTestAuthCommand(session), "", "export")
	if err != nil {
		t.Fatalf("auth export error = %v", err)
	}
	if !strings.Contains(stdout, bundle) {
		t.Fatalf("auth export stdout = %q, want bundle", stdout)
	}
	if !strings.Contains(stderr, "secret tokens") {
		t.Fatalf("auth export stderr = %q, want secret warning", stderr)
	}
	if session.exportCalls != 1 {
		t.Fatalf("Export calls = %d, want 1", session.exportCalls)
	}
}

func TestAuthImportReadsStdinWithoutEchoingSecret(t *testing.T) {
	secretBundle := `{"refresh_token":"secret-refresh"}`
	session := &stubAuthCommandSession{}

	stdout, stderr, err := executeAuthCommand(t, newTestAuthCommand(session), secretBundle, "import")
	if err != nil {
		t.Fatalf("auth import error = %v", err)
	}
	if session.imported != secretBundle {
		t.Fatalf("imported bundle = %q, want %q", session.imported, secretBundle)
	}
	if strings.Contains(stdout, secretBundle) || strings.Contains(stderr, secretBundle) {
		t.Fatalf("auth import echoed secret, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "imported") {
		t.Fatalf("auth import stdout = %q", stdout)
	}
}

func TestAuthImportReadsFile(t *testing.T) {
	secretBundle := `{"refresh_token":"file-secret"}`
	path := t.TempDir() + "/auth.json"
	if err := os.WriteFile(path, []byte(secretBundle), 0o600); err != nil {
		t.Fatalf("write auth bundle fixture: %v", err)
	}

	session := &stubAuthCommandSession{}
	_, _, err := executeAuthCommand(t, newTestAuthCommand(session), "", "import", "--file", path)
	if err != nil {
		t.Fatalf("auth import --file error = %v", err)
	}
	if session.imported != secretBundle {
		t.Fatalf("imported bundle = %q, want %q", session.imported, secretBundle)
	}
}

func newTestAuthCommand(session *stubAuthCommandSession) *cobra.Command {
	return newAuthCmd(func(config.Config) authCommandSession {
		return session
	})
}

func executeAuthCommand(t *testing.T, cmd *cobra.Command, stdin string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()

	configPath := writeTestConfigWith(t, config.Defaults())
	return executeAuthCommandWithConfig(t, cmd, configPath, stdin, args...)
}

func executeAuthCommandWithConfig(t *testing.T, cmd *cobra.Command, configPath string, stdin string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func writeTestConfigWith(t *testing.T, cfg config.Config) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	store, err := config.NewConfigStore(path)
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() default config error = %v", err)
	}
	return path
}

func findCommand(t *testing.T, commands []*cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, cmd := range commands {
		if cmd.Name() == name {
			return cmd
		}
	}
	t.Fatalf("command %q not found", name)
	return nil
}
