package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/cloudrunner"
	vercelprovider "github.com/multica-ai/multica/server/internal/cloudrunner/providers/vercel"
	loggerpkg "github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func main() {
	logger := loggerpkg.NewLogger("cloudrunner")

	cfg, err := cloudrunner.LoadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	client := cloudrunner.NewClient(cfg.ServerBaseURL, nil)
	client.SetToken(cfg.AuthToken)

	executor := strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_EXECUTOR"))
	if executor == "" {
		executor = "noop"
	}
	provider := cloudrunner.NewNoopProvider(logger)
	if executor == "vercel_sandbox" {
		clientMode := strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_CLIENT_MODE"))
		if clientMode == "" {
			clientMode = "cli"
		}

		var vercelClient vercelprovider.Client = vercelprovider.NewStubClient()
		if clientMode == "cli" {
			vercelClient = vercelprovider.NewClient(vercelprovider.ClientConfig{
				Binary:         envDefault("MULTICA_CLOUDRUNNER_VERCEL_BIN", "sandbox"),
				Args:           splitCSVEnv("MULTICA_CLOUDRUNNER_VERCEL_BIN_ARGS"),
				Token:          strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_TOKEN")),
				Project:        strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_PROJECT")),
				Team:           strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_TEAM")),
				BaseSnapshotID: strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_BASE_SNAPSHOT_ID")),
				Runtime:        envDefault("MULTICA_CLOUDRUNNER_VERCEL_RUNTIME", "node24"),
				Timeout:        durationEnv("MULTICA_CLOUDRUNNER_VERCEL_TIMEOUT", time.Hour),
				Workdir:        envDefault("MULTICA_CLOUDRUNNER_VERCEL_WORKDIR", "/workspace"),
			})
		}

		providerCfg := vercelprovider.ProviderConfig{
			Token:          strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_TOKEN")),
			Project:        strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_PROJECT")),
			Team:           strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_TEAM")),
			BaseSnapshotID: strings.TrimSpace(os.Getenv("MULTICA_CLOUDRUNNER_VERCEL_BASE_SNAPSHOT_ID")),
			Runtime:        envDefault("MULTICA_CLOUDRUNNER_VERCEL_RUNTIME", "node24"),
			Timeout:        durationEnv("MULTICA_CLOUDRUNNER_VERCEL_TIMEOUT", time.Hour),
			Workdir:        envDefault("MULTICA_CLOUDRUNNER_VERCEL_WORKDIR", "/workspace"),
			Command:        splitCommandEnv("MULTICA_CLOUDRUNNER_VERCEL_COMMAND", []string{"sh", "-lc", "echo \"Multica cloudrunner task ${MULTICA_TASK_ID}\""}),
		}
		provider = vercelprovider.NewProvider(vercelClient, logger, vercelprovider.WithProviderConfig(providerCfg))
	}

	runner := cloudrunner.NewRunner(cfg, client, provider)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		pool, err := pgxpool.New(context.Background(), dbURL)
		if err != nil {
			logger.Error("cloudrunner database connect", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		runner.WithSessionService(cloudrunner.NewSessionService(db.New(pool)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("cloudrunner stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("cloudrunner stopped")
}

func envDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func splitCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitCommandEnv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string{}, fallback...)
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return append([]string{}, fallback...)
	}
	return parts
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
