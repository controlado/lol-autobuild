package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/coachless"
	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/internal/recommend"
	"github.com/controlado/lol-autobuild/internal/secrets"
	"github.com/controlado/lol-autobuild/pkg/lolautobuild"
)

type syncFlags struct {
	ConfigPath  string
	Patch       string
	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	DryRun      bool
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dev",
		Short: "Development CLI for lol-autobuild",
	}

	root.AddCommand(syncCmd())
	return root
}

func syncCmd() *cobra.Command {
	flags := syncFlags{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run one synchronization cycle",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}

			level, err := zerolog.ParseLevel(cfg.LogLevel)
			if err != nil {
				level = zerolog.InfoLevel
			}
			zerolog.SetGlobalLevel(level)
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

			coachlessClient := coachless.NewClient(cfg.Coachless.APIBaseURL, time.Duration(cfg.Coachless.TimeoutSeconds)*time.Second)
			secretStore := secrets.NewKeyringStore(cfg.Secrets.ServiceName)
			lcuClient := lcu.NewClient(cfg.LCU.Enabled, cfg.LCU.LockfilePath)

			provider := auth.NewProvider(
				coachlessClient,
				secretStore,
				auth.BrowserSource{LoginURL: "https://coachless.gg/login-area/login"},
				auth.EnvManualSource{},
				auth.ProviderOptions{
					AutoEnabled:           cfg.Auth.AutoEnabled,
					ManualFallbackEnabled: cfg.Auth.ManualFallbackEnabled,
					TokenSkew:             time.Duration(cfg.Auth.TokenSkewSeconds) * time.Second,
				},
			)

			svc, err := lolautobuild.NewService(lolautobuild.ServiceDeps{
				Coachless:   coachlessClient,
				Tokens:      provider,
				LCU:         lcuClient,
				Recommender: recommend.NewEngine(),
				Policy: lolautobuild.RecommendationPolicy{
					MinOccurrence: cfg.Recommendation.MinOccurrence,
					TopItems:      cfg.Recommendation.TopItems,
					TopSpells:     cfg.Recommendation.TopSpells,
				},
			})
			if err != nil {
				return err
			}

			req := lolautobuild.SyncRequest{
				Patch:       flags.Patch,
				ApplyItems:  flags.ApplyItems,
				ApplyRunes:  flags.ApplyRunes,
				ApplySpells: flags.ApplySpells,
				DryRun:      flags.DryRun,
			}

			result, err := svc.Sync(cmd.Context(), req)
			if err != nil {
				notConfigured := lcu.ErrNotConfigured
				if errors.Is(err, notConfigured) {
					log.Warn().Err(err).Msg("LCU is not configured")
				}

				return err
			}

			encoded, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(encoded))
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.ConfigPath, "config", "config.example.yaml", "Path to YAML configuration file")
	cmd.Flags().StringVar(&flags.Patch, "patch", "", "Patch label (e.g. 16.7). Empty = latest")
	cmd.Flags().BoolVar(&flags.ApplyItems, "apply-items", true, "Apply item set")
	cmd.Flags().BoolVar(&flags.ApplyRunes, "apply-runes", true, "Apply rune page")
	cmd.Flags().BoolVar(&flags.ApplySpells, "apply-spells", true, "Apply summoner spells")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", true, "Do not apply changes to LCU")

	return cmd
}
