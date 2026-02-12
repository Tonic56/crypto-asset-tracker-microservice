package kaffka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/models"
	"github.com/segmentio/kafka-go"
)

type consumer struct {
	reader *kafka.Reader
	config kafkaConfig
}

func NewConsumer(ctx context.Context, cfg kafkaConfig) *consumer {
	slog.Info("Start initializing consumer",
		"brockers", cfg.Brokers,
		"topic", cfg.Topic,
		"groupID", cfg.GroupID,
	)

	for i := 0; i < cfg.MaxRetries; i++ {
		conn, err := kafka.DialContext(ctx, "tcp", cfg.Brokers[0])
		if err != nil {
			slog.Warn("not available yet",
				"attempt", i+1,
				"error", err,
				"delay", cfg.RetryDelay,
			)
			time.Sleep(cfg.RetryDelay)
			continue
		}

		brockers, err := conn.Brokers()
		if err != nil {
			conn.Close()
			slog.Warn("Could not get metadata",
				"attempt", i+1,
				"error", err,
				"delay", cfg.RetryDelay,
			)
			time.Sleep(cfg.RetryDelay)
			continue
		}

		conn.Close()
		slog.Info("is available", "brokers", len(brockers))
		break
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           cfg.Brokers,
		Topic:             cfg.Topic,
		GroupID:           cfg.GroupID,
		MinBytes:          10e3, 
		MaxBytes:          10e6, 
		CommitInterval:    time.Second,
		SessionTimeout:    cfg.SessionTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
		StartOffset:       kafka.LastOffset,
		Logger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug("Kafka reader", "message", msg)
		}),
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Error("Kafka reader error", "message", fmt.Sprintf(msg, args...))
		}),
	})

	slog.Info("✅ Kafka consumer initialized successfully")

	return &consumer{
		reader: reader,
		config: cfg,
	}
}

func (c *consumer) Start(ctx context.Context, wg *sync.WaitGroup, outChan chan<- models.KafkaMsg) {
	defer wg.Done()
	defer c.reader.Close()
	defer close(outChan)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got interruption signal, stopping Kafka Consumer")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Could not read message from Kafka Producer", "error", err)
				time.Sleep(c.config.RetryDelay)
				continue
			}

			var kafMsg models.KafkaMsg
			if err := json.Unmarshal(msg.Value, &kafMsg); err != nil {
				slog.Error("Failed to parse Kafka message into struct KafkaMsg",
					"error", err,
					"offset", msg.Offset,
					"partition", msg.Partition,
				)

				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					slog.Error("Failed to commit invalid message", "error", err)
				}

				continue
			}

			select {
			case <-ctx.Done():
				slog.Info("Got interruption signal, stopping Kafka Consumer")
				return
			case outChan <- kafMsg:
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					slog.Error("Failed to commit message", "error", err)
				} else {
					slog.Debug("Valid message commited succesfully",
						"offset", msg.Offset,
						"patrition", msg.Partition,
						"symbol", kafMsg.Symbol)
				}
			}
		}
	}
}
