package repository

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/adapters/clkhouse"
)

type Repository struct {
	client      *clkhouse.Client
	cfg         clkhouse.ClickHouseConfig
	batchBuffer []models.KafkaMsg
	batchTimer  *time.Timer
	mu          sync.Mutex
}

func NewRepository(client *clkhouse.Client, cfg clkhouse.ClickHouseConfig) *Repository {
	return &Repository{
		client:      client,
		cfg:         cfg,
		batchBuffer: make([]models.KafkaMsg, 0, cfg.BatchSize),
		batchTimer:  time.NewTimer(cfg.BatchTimeout),
	}
}

func (r *Repository) CreateTable(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("clickhouse clietn is nil")
	}

	dbCreateQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", r.cfg.Database)
	slog.Info("Creating database if not exists", "database", r.cfg.Database)
	if err := r.client.Exec(ctx, dbCreateQuery); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			message_id String,
			event_type String,
			event_time DateTime64(3),
			receive_time DateTime64(3),
			symbol String,
			close_price Decimal64(8),
			open_price Decimal64(8),
			high_price Decimal64(8),
			low_price Decimal64(8),
			change_price Decimal64(8),
			change_percent Decimal64(8)
		) ENGINE = MergeTree()
		ORDER BY (symbol, event_time)
		PARTITION BY toYYYYMM(event_time)
		SETTINGS index_granularity = 8192
	`, r.cfg.Database, r.cfg.Table)

	slog.Info("Creating table if not exists", "table", r.cfg.Table)

	if err := r.client.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	slog.Info("Table ready", "table", r.cfg.Table)
	return nil
}

func (r *Repository) BatchInsert(
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChan <-chan models.KafkaMsg,
) {
	defer wg.Done()

	slog.Info("🚀 Batch inserter started",
		"batch_size", r.cfg.BatchSize,
		"batch_timeout", r.cfg.BatchTimeout,
	)

	for {
		select {
		case <-ctx.Done():
			r.insertRemainingMsgs(ctx)
			return

		case <-r.batchTimer.C:
			r.insertRemainingMsgs(ctx)
			r.batchTimer.Reset(r.cfg.BatchTimeout)

		case msg, ok := <-inputChan:
			if !ok {
				r.insertRemainingMsgs(ctx)
				return
			}

			r.mu.Lock()
			r.batchBuffer = append(r.batchBuffer, msg)

			if len(r.batchBuffer) >= r.cfg.BatchSize {
				slog.Debug("Batch full, inserting", "size", len(r.batchBuffer))
				if err := r.insert(ctx); err != nil {
					slog.Error("Failed to insert batch", "error", err)
				}
				r.batchTimer.Reset(r.cfg.BatchTimeout)
			}
			r.mu.Unlock()
		}
	}
}

func (r *Repository) insert(ctx context.Context) error {
	if len(r.batchBuffer) == 0 {
		return nil
	}

	start := time.Now()
	query := fmt.Sprintf(`
		INSERT INTO %s.%s (
		message_id, event_type, event_time, receive_time, symbol, close_price, open_price, high_price, low_price, change_price, change_percent)`, r.cfg.Database, r.cfg.Table)

	batch, err := r.client.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, msg := range r.batchBuffer {
		err := batch.Append(
			msg.MessageID,
			msg.EventType,
			time.UnixMilli(msg.EventTime),
			time.UnixMilli(msg.RecvTime),
			msg.Symbol,
			msg.ClosePrice,
			msg.OpenPrice,
			msg.HighPrice,
			msg.LowPrice,
			msg.ChangePrice,
			msg.ChangePercent,
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to sent batch: %w", err)
	}

	finish := time.Since(start)
	slog.Info("✍️ Batch inserted successfully",
		"size", len(r.batchBuffer),
		"duration", finish,
	)

	r.batchBuffer = r.batchBuffer[:0]
	return nil
}

func (r *Repository) insertRemainingMsgs(ctx context.Context) {
	r.mu.Lock()
	if len(r.batchBuffer) > 0 {
		slog.Info("Inserting remaining messages before shutdown",
			"size", len(r.batchBuffer),
		)
		if err := r.insert(ctx); err != nil {
			slog.Error("Failed to insert remaining batch",
				"error", err,
			)
		}
	}
	r.mu.Unlock()
	slog.Info("Batch inserter stopped")
}
