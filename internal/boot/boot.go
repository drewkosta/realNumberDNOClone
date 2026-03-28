package boot

import (
	"log/slog"
	"os"

	"realNumberDNOClone/internal/config"
	"realNumberDNOClone/internal/db"
)

type App struct {
	Cfg    *config.Config
	DB     *db.DB
	Logger *slog.Logger
}

func Init(env string) (*App, error) {
	cfg, err := config.Load(env)
	if err != nil {
		return nil, err
	}

	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	database, err := db.Initialize(cfg)
	if err != nil {
		return nil, err
	}

	return &App{Cfg: cfg, DB: database, Logger: logger}, nil
}

func (a *App) Close() {
	a.DB.Close()
}
