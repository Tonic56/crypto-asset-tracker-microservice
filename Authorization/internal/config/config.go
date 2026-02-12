package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env      string `env:"ENV" env-default:"local"`
	GRPC     GRPCConfig
	Database DBConfig
	Token    TokenConfig
}

type GRPCConfig struct {
	Port             uint16        `env:"GRPC_PORT" env-default:"50051"`
	Timeout          time.Duration `env:"GRPC_TIMEOUT" env-default:"1h"`
	EnableReflection bool          `env:"GRPC_ENABLE_REFLECTION" env-default:"true"`
}

type DBConfig struct {
	Host     string `env:"POSTGRES_HOST" env-default:"localhost"`
	Port     uint16 `env:"POSTGRES_PORT" env-default:"5432"`
	User     string `env:"POSTGRES_USER" env-default:"postgres"`
	Password string `env:"POSTGRES_PASSWORD" env-default:"postgres"`
	DBName   string `env:"POSTGRES_DB" env-default:"users"`
}

type TokenConfig struct {
	Secret               string        `env:"KEY_SECRET" env-required:"true"`
	AccessToken          time.Duration `env:"ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshToken         time.Duration `env:"REFRESH_TOKEN_TTL" env-default:"168h"`
	TokenCleanupInterval time.Duration `env:"TOKEN_CLEANUP_INTERVAL" env-default:"1h"`
}

func MustLoad() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading from environment variables")
	}

	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("no .env file found, reading from environment variables")
		os.Exit(1)
	}

	return &cfg
}
