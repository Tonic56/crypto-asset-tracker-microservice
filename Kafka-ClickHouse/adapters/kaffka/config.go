package kaffka

import (
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/lib/getenv"
)

type kafkaConfig struct {
	Brokers           []string
	Topic             string
	GroupID           string
	MaxRetries        int
	RetryDelay        time.Duration
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
}

func LoadKafkaConfig() kafkaConfig {
	return kafkaConfig{
		Brokers:           getenv.GetSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:             getenv.GetString("KAFKA_TOPIC", "binance.miniticker"),
		GroupID:           getenv.GetString("KAFKA_GROUP_ID", "clickhouse-consumer-group"),
		MaxRetries:        getenv.GetInt("KAFKA_MAX_RETRIES", 5),
		RetryDelay:        getenv.GetDuration("KAFKA_RETRY_DELAY", 2*time.Second),
		SessionTimeout:    getenv.GetDuration("KAFKA_SESSION_TIMEOUT", 10*time.Second),
		HeartbeatInterval: getenv.GetDuration("KAFKA_HEARTBEAT_INTERVAL", 3*time.Second),
	}
}
