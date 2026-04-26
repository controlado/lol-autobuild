package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
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

type runFlags struct {
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
	root.AddCommand(watchCmd())
	return root
}

func syncCmd() *cobra.Command {
	flags := runFlags{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run one synchronization cycle",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigAndLogging(flags.ConfigPath)
			if err != nil {
				return err
			}

			svc, err := buildService(cfg)
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
				warnIfLCUNotConfigured(err)
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

	bindRunFlags(cmd, &flags)
	return cmd
}

func watchCmd() *cobra.Command {
	flags := runFlags{}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch LCU events and run synchronization continuously",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigAndLogging(flags.ConfigPath)
			if err != nil {
				return err
			}

			svc, err := buildService(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			log.Info().
				Dur("debounce", time.Duration(cfg.Watch.DebounceMillis)*time.Millisecond).
				Dur("reconnect_delay", time.Duration(cfg.Watch.ReconnectDelayMillis)*time.Millisecond).
				Msg("watch started; press CTRL+C to stop")

			err = svc.Watch(ctx, lolautobuild.WatchRequest{
				Patch:       flags.Patch,
				ApplyItems:  flags.ApplyItems,
				ApplyRunes:  flags.ApplyRunes,
				ApplySpells: flags.ApplySpells,
				DryRun:      flags.DryRun,
				Debounce:    time.Duration(cfg.Watch.DebounceMillis) * time.Millisecond,
				OnCycle:     logWatchCycle,
			})
			if err != nil {
				warnIfLCUNotConfigured(err)
				return err
			}

			log.Info().Msg("watch stopped")
			return nil
		},
	}

	bindRunFlags(cmd, &flags)
	return cmd
}

func bindRunFlags(cmd *cobra.Command, flags *runFlags) {
	cmd.Flags().StringVar(&flags.ConfigPath, "config", "config.example.yaml", "Path to YAML configuration file")
	cmd.Flags().StringVar(&flags.Patch, "patch", "", "Patch label (e.g. 16.7). Empty = latest")
	cmd.Flags().BoolVar(&flags.ApplyItems, "apply-items", true, "Apply item set")
	cmd.Flags().BoolVar(&flags.ApplyRunes, "apply-runes", true, "Apply rune page")
	cmd.Flags().BoolVar(&flags.ApplySpells, "apply-spells", true, "Apply summoner spells")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", true, "Do not apply changes to LCU")
}

func loadConfigAndLogging(configPath string) (config.Config, error) {
	configStore, err := config.NewConfigStore(configPath)
	if err != nil {
		return config.Config{}, err
	}

	cfg, err := configStore.Load()
	if err != nil {
		return config.Config{}, err
	}

	if err := loadEnvFileFromConfig(configPath, cfg); err != nil {
		return config.Config{}, err
	}

	configureLogging(cfg.LogLevel)
	return cfg, nil
}

func configureLogging(rawLevel string) {
	level, err := zerolog.ParseLevel(rawLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func buildService(cfg config.Config) (lolautobuild.Service, error) {
	coachlessClient := coachless.NewClient(cfg.Coachless.APIBaseURL, time.Duration(cfg.Coachless.TimeoutSeconds)*time.Second)
	secretStore := secrets.NewKeyringStore(cfg.Secrets.ServiceName)
	lcuClient := lcu.NewClient(cfg.LCU.Enabled, cfg.LCU.LockfilePath)
	lcuClient.WatchReconnectDelay = time.Duration(cfg.Watch.ReconnectDelayMillis) * time.Millisecond

	provider := auth.NewProvider(
		coachlessClient,
		secretStore,
		auth.BrowserSource{
			LoginURL:       "https://coachless.gg/login-area/login",
			AcquireTimeout: 3 * time.Minute,
		},
		auth.EnvManualSource{},
		auth.ProviderOptions{
			AutoEnabled:           cfg.Auth.AutoEnabled,
			ManualFallbackEnabled: cfg.Auth.ManualFallbackEnabled,
			TokenSkew:             time.Duration(cfg.Auth.TokenSkewSeconds) * time.Second,
		},
	)

	return lolautobuild.NewService(lolautobuild.ServiceDeps{
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
}

func warnIfLCUNotConfigured(err error) {
	if errors.Is(err, lcu.ErrNotConfigured) {
		log.Warn().Err(err).Msg("LCU is not configured")
	}
}

func logWatchCycle(cycle lolautobuild.WatchCycle) {
	logger := log.Info()
	if cycle.Err != nil {
		logger = log.Warn().Err(cycle.Err)
	}

	logger = logger.Str("trigger", string(cycle.Trigger))
	if cycle.EventType != "" {
		logger = logger.Str("event_type", cycle.EventType)
	}
	if cycle.EventURI != "" {
		logger = logger.Str("event_uri", cycle.EventURI)
	}

	if cycle.Err != nil {
		logger.Msg("watch cycle failed")
		return
	}

	if cycle.Result == nil {
		logger.Msg("watch cycle completed with no sync result")
		return
	}

	logger = logger.
		Int("champion_id", cycle.Result.DetectedChampionID).
		Str("position", cycle.Result.DetectedPosition).
		Int("queue_id", cycle.Result.DetectedQueueID).
		Bool("item_set_applied", cycle.Result.ItemSetApplied).
		Bool("rune_page_applied", cycle.Result.RunePageApplied).
		Bool("spells_applied", cycle.Result.SpellsApplied)

	if len(cycle.Result.Warnings) > 0 {
		logger = logger.Strs("warnings", cycle.Result.Warnings)
	}

	logger.Msg("watch cycle completed")
}
