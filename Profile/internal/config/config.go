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
	HTTP     HTTPConfig
	Database DBConfig
	Security SecConfig
}

type GRPCConfig struct {
	Port             uint16        `env:"GRPC_PORT" env-default:"50052"`
	Timeout          time.Duration `env:"GRPC_TIMEOUT" env-default:"1h"`
	EnableReflection bool          `env:"GRPC_ENABLE_REFLECTION" env-default:"true"`
	AuthServiceAddr  string        `env:"AUTH_SERVICE_ADDR" env-required:"true"`
}

type HTTPConfig struct {
	Port    uint16        `env:"HTTP_PORT" env-default:"8080"`
	Timeout time.Duration `env:"HTTP_TIMEOUT" env-default:"30s"`
}

type DBConfig struct {
	Host     string `env:"POSTGRES_HOST" env-default:"localhost"`
	Port     uint16 `env:"POSTGRES_PORT" env-default:"5433"`
	User     string `env:"POSTGRES_USER" env-default:"postgres"`
	Password string `env:"POSTGRES_PASSWORD" env-default:"postgres"`
	DBName   string `env:"POSTGRES_DB" env-default:"profile_db"`
}

type SecConfig struct {
	JWTSecret string `env:"JWT_SECRET" env-required:"true"`
}

func MustLoad() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading from environment variables")
	}

	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read environment variables", "error", err)
		os.Exit(1)
	}

	return &cfg
}