package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/controlado/lol-autobuild/internal/app"
	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/autobuild"
	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
	"github.com/controlado/lol-autobuild/internal/autobuild/recommend"
	"github.com/controlado/lol-autobuild/internal/buildinfo"
	"github.com/controlado/lol-autobuild/internal/coachless"
	"github.com/controlado/lol-autobuild/internal/config"
	"github.com/controlado/lol-autobuild/internal/lcu"
	"github.com/controlado/lol-autobuild/internal/secrets"
	"github.com/controlado/lol-autobuild/internal/ui"
	"github.com/controlado/lol-autobuild/internal/update"
)

const (
	defaultConfigPath = "config.yaml"
	coachlessLoginURL = "https://coachless.gg/login-area/login"
)

type executionFlags struct {
	ConfigPath  string
	Patch       string
	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	DryRun      bool
}

func main() {
	cobra.MousetrapHelpText = "" // allows windows explorer launch

	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "lol-autobuild",
		Short: "Automate League of Legends setup from Coachless data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUI(cmd.Context(), defaultConfigPath)
		},
	}

	root.AddCommand(uiCmd())
	root.AddCommand(authCmd())
	root.AddCommand(syncCmd())
	root.AddCommand(watchCmd())
	return root
}

func uiCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the local settings UI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUI(cmd.Context(), configPath)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", defaultConfigPath, "Path to YAML configuration file")
	return cmd
}

func syncCmd() *cobra.Command {
	flags := executionFlags{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run one synchronization cycle",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigAndLogging(flags.ConfigPath)
			if err != nil {
				return err
			}

			svc, err := buildService(cfg)
			if err != nil {
				return err
			}

			result, err := svc.Sync(cmd.Context(), syncRequestFromConfigAndFlags(cfg, flags, executionFlagChangesFromCommand(cmd)))
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
	flags := executionFlags{}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch LCU events and run synchronization continuously",
		Args:  cobra.NoArgs,
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

			req := watchRequestFromConfigAndFlags(cfg, flags, executionFlagChangesFromCommand(cmd))
			req.OnCycle = logWatchCycle
			req.OnNotice = logWatchNotice
			err = svc.Watch(ctx, req)
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

func runUI(ctx context.Context, configPath string) error {
	cfg, err := loadUIConfigAndLogging(configPath)
	if err != nil {
		return err
	}

	fileConfigStore, err := config.NewConfigStore(configPath)
	if err != nil {
		return err
	}
	appConfigStore := newAppConfigStore(fileConfigStore, cfg)

	application, err := app.New(app.Options{
		ServiceFactory: func(appCfg app.RuntimeConfig) (autobuild.Service, error) {
			return buildService(appConfigStore.configFor(appCfg))
		},
		LCUStatus: func(ctx context.Context, appCfg app.RuntimeConfig) app.LCUStatus {
			return appLCUStatusFromLCU(checkLCUStatus(ctx, appConfigStore.configFor(appCfg)))
		},
		ChampSelect: func(ctx context.Context, appCfg app.RuntimeConfig) (domain.ChampSelectState, error) {
			cfg := appConfigStore.configFor(appCfg)
			return lcu.NewClient(cfg.LCU.Enabled, cfg.LCU.LockfilePath).DetectEnemyChampions(ctx)
		},
		UpdateChecker: appUpdateChecker{
			source: update.NewGitHubChecker(update.Options{CurrentVersion: buildinfo.Version}),
		},
		CoachlessAuth:    appCoachlessAuthSession{session: buildCoachlessAuthSession(cfg)},
		ConfigStore:      appConfigStore,
		RuntimeConfig:    runtimeConfigFromConfig(cfg),
		MessageFromError: appMessageFromErr,
	})
	if err != nil {
		return err
	}

	server, err := ui.NewServer(ui.Options{
		App:         application,
		OpenBrowser: ui.OpenBrowser,
		Out:         os.Stdout,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return server.Run(ctx)
}

func bindRunFlags(cmd *cobra.Command, flags *executionFlags) {
	cmd.Flags().StringVar(&flags.ConfigPath, "config", defaultConfigPath, "Path to YAML configuration file")
	cmd.Flags().StringVar(&flags.Patch, "patch", "", "Patch label (e.g. 16.7). Empty = latest")
	cmd.Flags().BoolVar(&flags.ApplyItems, "apply-items", true, "Apply item set")
	cmd.Flags().BoolVar(&flags.ApplyRunes, "apply-runes", true, "Apply rune page")
	cmd.Flags().BoolVar(&flags.ApplySpells, "apply-spells", true, "Apply summoner spells")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", config.Defaults().Sync.DryRun, "Do not apply changes to LCU")
}

type executionFlagChanges struct {
	Patch       bool
	ApplyItems  bool
	ApplyRunes  bool
	ApplySpells bool
	DryRun      bool
}

func executionFlagChangesFromCommand(cmd *cobra.Command) executionFlagChanges {
	flags := cmd.Flags()
	return executionFlagChanges{
		Patch:       flags.Changed("patch"),
		ApplyItems:  flags.Changed("apply-items"),
		ApplyRunes:  flags.Changed("apply-runes"),
		ApplySpells: flags.Changed("apply-spells"),
		DryRun:      flags.Changed("dry-run"),
	}
}

func syncRequestFromConfigAndFlags(cfg config.Config, flags executionFlags, changed executionFlagChanges) autobuild.SyncRequest {
	req := autobuild.SyncRequest{
		Patch:              cfg.Sync.Patch,
		PatchAdditionsMode: cfg.Sync.PatchAdditionsMode,
		PatchAdditions:     cfg.Sync.PatchAdditions,
		LeagueTierPreset:   cfg.Sync.LeagueTierPreset,
		Regions:            slices.Clone(cfg.Sync.Regions),
		ApplyItems:         cfg.Sync.ApplyItems,
		ApplyRunes:         cfg.Sync.ApplyRunes,
		ApplySpells:        cfg.Sync.ApplySpells,
		KeepFlash:          cfg.Sync.KeepFlash,
		DryRun:             cfg.Sync.DryRun,
	}
	if changed.Patch {
		req.Patch = flags.Patch
	}
	if changed.ApplyItems {
		req.ApplyItems = flags.ApplyItems
	}
	if changed.ApplyRunes {
		req.ApplyRunes = flags.ApplyRunes
	}
	if changed.ApplySpells {
		req.ApplySpells = flags.ApplySpells
	}
	if changed.DryRun {
		req.DryRun = flags.DryRun
	}
	return req
}

func watchRequestFromConfigAndFlags(cfg config.Config, flags executionFlags, changed executionFlagChanges) autobuild.WatchRequest {
	syncReq := syncRequestFromConfigAndFlags(cfg, flags, changed)
	return autobuild.WatchRequest{
		Patch:              syncReq.Patch,
		PatchAdditionsMode: syncReq.PatchAdditionsMode,
		PatchAdditions:     syncReq.PatchAdditions,
		LeagueTierPreset:   syncReq.LeagueTierPreset,
		Regions:            slices.Clone(syncReq.Regions),
		ApplyItems:         syncReq.ApplyItems,
		ApplyRunes:         syncReq.ApplyRunes,
		ApplySpells:        syncReq.ApplySpells,
		KeepFlash:          syncReq.KeepFlash,
		DryRun:             syncReq.DryRun,
		Debounce:           time.Duration(cfg.Watch.DebounceMillis) * time.Millisecond,
	}
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

func loadUIConfigAndLogging(configPath string) (config.Config, error) {
	cfg, err := loadConfigAndLogging(configPath)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return config.Config{}, err
	}

	cfg = config.Defaults()
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

func buildService(cfg config.Config) (autobuild.Service, error) {
	coachlessClient := coachless.NewClient(cfg.Coachless.APIBaseURL, time.Duration(cfg.Coachless.TimeoutSeconds)*time.Second)
	secretStore := secrets.NewKeyringStore(cfg.Secrets.ServiceName)
	lcuClient := lcu.NewClient(cfg.LCU.Enabled, cfg.LCU.LockfilePath)
	lcuClient.WatchReconnectDelay = time.Duration(cfg.Watch.ReconnectDelayMillis) * time.Millisecond

	provider := auth.NewProvider(
		coachlessClient,
		secretStore,
		auth.BrowserSource{
			LoginURL:       coachlessLoginURL,
			AcquireTimeout: 3 * time.Minute,
		},
		auth.EnvManualSource{},
		auth.ProviderOptions{
			AutoEnabled:           cfg.Auth.AutoEnabled,
			ManualFallbackEnabled: cfg.Auth.ManualFallbackEnabled,
			TokenSkew:             time.Duration(cfg.Auth.TokenSkewSeconds) * time.Second,
		},
	)

	return autobuild.NewService(autobuild.ServiceDeps{
		Coachless:   coachlessClient,
		Tokens:      provider,
		LCU:         lcuClient,
		Recommender: recommend.NewEngine(),
		Policy: autobuild.RecommendationPolicy{
			MinOccurrence: cfg.Recommendation.MinOccurrence,
			TopItems:      cfg.Recommendation.TopItems,
			TopSpells:     cfg.Recommendation.TopSpells,
		},
	})
}

func buildCoachlessAuthSession(cfg config.Config) *auth.CoachlessSession {
	coachlessClient := coachless.NewClient(cfg.Coachless.APIBaseURL, time.Duration(cfg.Coachless.TimeoutSeconds)*time.Second)
	secretStore := secrets.NewKeyringStore(cfg.Secrets.ServiceName)

	return auth.NewCoachlessSession(
		coachlessClient,
		secretStore,
		auth.BrowserSource{
			LoginURL:       coachlessLoginURL,
			AcquireTimeout: 3 * time.Minute,
		},
		auth.CoachlessSessionOptions{
			TokenSkew: time.Duration(cfg.Auth.TokenSkewSeconds) * time.Second,
		},
	)
}

func checkLCUStatus(ctx context.Context, cfg config.Config) lcu.ConnectionStatus {
	return lcu.NewClient(cfg.LCU.Enabled, cfg.LCU.LockfilePath).ConnectionStatus(ctx)
}

func warnIfLCUNotConfigured(err error) {
	if errors.Is(err, lcu.ErrNotConfigured) {
		log.Warn().Err(err).Msg("LCU is not configured")
	}
}

func logWatchCycle(cycle autobuild.WatchCycle) {
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
		Str("champion_name", cycle.Result.DetectedChampionName).
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

func logWatchNotice(notice autobuild.WatchNotice) {
	logger := log.Info()
	if notice.Err != nil {
		logger = log.Warn().Err(notice.Err)
	}

	logger = logger.Str("kind", string(notice.Kind))
	if notice.Source != "" {
		logger = logger.Str("source", notice.Source)
	}
	if notice.URI != "" {
		logger = logger.Str("event_uri", notice.URI)
	}
	if notice.Phase != "" {
		logger = logger.Str("phase", notice.Phase)
	}
	if notice.ConnectionID > 0 {
		logger = logger.Int("connection_id", notice.ConnectionID)
	}

	if notice.Message != "" {
		logger.Msg(notice.Message)
		return
	}
	logger.Msg("watch notice")
}
