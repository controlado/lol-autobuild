package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/config"
)

type authCommandSession interface {
	Status(context.Context) auth.CoachlessSessionState
	Login(context.Context) error
	Logout(context.Context) error
	Export(context.Context) (string, error)
	Import(context.Context, string) error
}

type authCommandSessionFactory func(config.Config) authCommandSession

type authStatusOutput struct {
	Status    auth.CoachlessSessionStatus `json:"status"`
	Plan      auth.CoachlessPlan          `json:"plan"`
	ExpiresAt *time.Time                  `json:"expires_at,omitempty"`
	Message   string                      `json:"message,omitempty"`
}

func authCmd() *cobra.Command {
	return newAuthCmd(func(cfg config.Config) authCommandSession { return buildCoachlessAuthSession(cfg) })
}

func newAuthCmd(sessionFactory authCommandSessionFactory) *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Coachless authentication",
		Args:  cobra.NoArgs,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath, "Path to YAML configuration file")

	cmd.AddCommand(authStatusCmd(&configPath, sessionFactory))
	cmd.AddCommand(authLoginCmd(&configPath, sessionFactory))
	cmd.AddCommand(authLogoutCmd(&configPath, sessionFactory))
	cmd.AddCommand(authExportCmd(&configPath, sessionFactory))
	cmd.AddCommand(authImportCmd(&configPath, sessionFactory))

	return cmd
}

func authStatusCmd(configPath *string, sessionFactory authCommandSessionFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Coachless authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := loadAuthCommandSession(*configPath, sessionFactory)
			if err != nil {
				return err
			}

			status := session.Status(cmd.Context())
			out := authStatusOutput{
				Status:    status.Status,
				Plan:      status.Plan,
				ExpiresAt: status.ExpiresAt,
				Message:   status.Message,
			}

			encoded, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("encode auth status: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return err
		},
	}
}

func authLoginCmd(configPath *string, sessionFactory authCommandSessionFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Sign in to Coachless",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := loadAuthCommandSession(*configPath, sessionFactory)
			if err != nil {
				return err
			}
			if err := session.Login(cmd.Context()); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Coachless authentication saved.")
			return err
		},
	}
}

func authLogoutCmd(configPath *string, sessionFactory authCommandSessionFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign out of Coachless",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := loadAuthCommandSession(*configPath, sessionFactory)
			if err != nil {
				return err
			}
			if err := session.Logout(cmd.Context()); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Coachless authentication cleared.")
			return err
		},
	}
}

func authExportCmd(configPath *string, sessionFactory authCommandSessionFactory) *cobra.Command {
	return &cobra.Command{
		Use:    "export",
		Short:  "Export Coachless authentication",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := loadAuthCommandSession(*configPath, sessionFactory)
			if err != nil {
				return err
			}

			bundle, err := session.Export(cmd.Context())
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Warning: exported Coachless authentication contains secret tokens. Keep it private."); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), bundle)
			return err
		},
	}
}

func authImportCmd(configPath *string, sessionFactory authCommandSessionFactory) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:    "import",
		Short:  "Import Coachless authentication",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := loadAuthCommandSession(*configPath, sessionFactory)
			if err != nil {
				return err
			}

			raw, err := readAuthImportBundle(cmd, filePath)
			if err != nil {
				return err
			}
			if err := session.Import(cmd.Context(), raw); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Coachless authentication imported.")
			return err
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to Coachless authentication bundle")
	return cmd
}

func loadAuthCommandSession(configPath string, sessionFactory authCommandSessionFactory) (authCommandSession, error) {
	cfg, err := loadConfigAndLogging(configPath)
	if err != nil {
		return nil, err
	}

	session := sessionFactory(cfg)
	if session == nil {
		return nil, fmt.Errorf("coachless authentication is unavailable")
	}
	return session, nil
}

func readAuthImportBundle(cmd *cobra.Command, filePath string) (string, error) {
	if strings.TrimSpace(filePath) != "" {
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read auth bundle file: %w", err)
		}
		return string(raw), nil
	}

	raw, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("read auth bundle stdin: %w", err)
	}
	return string(raw), nil
}
