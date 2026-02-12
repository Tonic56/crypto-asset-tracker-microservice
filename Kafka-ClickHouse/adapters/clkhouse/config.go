package clkhouse

import (
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/lib/getenv"
)

type ClickHouseConfig struct {
	Addr         string
	Database     string
	Table        string
	Username     string
	Password     string
	BatchSize    int
	BatchTimeout time.Duration
	MaxRetries   int
	DialTimeout  time.Duration
}

func LoadClickHouseConfig() ClickHouseConfig {
	return ClickHouseConfig{
		Addr:         getenv.GetString("CLICKHOUSE_ADDR", "localhost:9000"),
		Database:     getenv.GetString("CLICKHOUSE_DATABASE", "crypto"),
		Table:        getenv.GetString("CLICKHOUSE_TABLE", "market_tickers"),
		Username:     getenv.GetString("CLICKHOUSE_USERNAME", "default"),
		Password:     getenv.GetString("CLICKHOUSE_PASSWORD", ""),
		BatchSize:    getenv.GetInt("BATCH_SIZE", 1000),
		BatchTimeout: getenv.GetDuration("BATCH_TIMEOUT", 5*time.Second),
		MaxRetries:   getenv.GetInt("CLICKHOUSE_MAX_RETRIES", 3),
		DialTimeout:  getenv.GetDuration("CLICKHOUSE_DIAL_TIMEOUT", 10*time.Second),
	}
}
