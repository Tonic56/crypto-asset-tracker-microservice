package reddis

import (
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/lib/getenv"
)

type redisConfig struct {
	Addr        string
	Password    string
	DBnum       int
	MaxRetries  int
	TTL         time.Duration
	RetryDelay  time.Duration
	PingTimeout time.Duration
}

func LoadRedisConfig() redisConfig {
	return redisConfig{
		Addr:        getenv.GetString("REDIS_ADDR", "redis:6379"),
		Password:    getenv.GetString("REDIS_PWD", ""),
		DBnum:       getenv.GetInt("REDIS_DB", 0),
		MaxRetries:  getenv.GetInt("REDIS_MAX_RETRIES", 10),
		TTL:         getenv.GetTime("REDIS_TTL", 30*time.Second),
		RetryDelay:  getenv.GetTime("REDIS_RETRY_DELAY", 2*time.Second),
		PingTimeout: getenv.GetTime("REDIS_PING_TIMEOUT", 5*time.Second),
	}
}
