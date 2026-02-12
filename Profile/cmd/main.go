package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/app"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/config"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)
	log.Info("starting profile service", slog.String("env", cfg.Env))

	application := app.New(log, cfg)

	go func() {
		if err := application.Run(); err != nil {
			log.Error("failed to run app", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop

	log.Info("stopping profile service...")
	application.Stop()
	log.Info("profile service stoped.")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger
	switch env {
	case "local":
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case "dev":
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case "prod":
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
