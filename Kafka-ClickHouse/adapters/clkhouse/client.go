package clkhouse

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Client struct {
	conn driver.Conn
	cfg  ClickHouseConfig
}

func NewClient(ctx context.Context, cfg ClickHouseConfig) *Client {
	slog.Debug("Set up connection to ClickHouse",
		"address", cfg.Addr,
		"database", cfg.Database,
	)

	clientCtx, cancel := context.WithCancel(ctx)

	var conn driver.Conn
	var err error

	for i := 0; i < cfg.MaxRetries; i++ {
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{cfg.Addr},
			Auth: clickhouse.Auth{
				Database: "default",
				Username: cfg.Username,
				Password: cfg.Password,
			},
			DialTimeout: cfg.DialTimeout,
			Compression: &clickhouse.Compression{
				Method: clickhouse.CompressionLZ4,
			},
			Settings: clickhouse.Settings{
				"max_execution_time": 60,
			},
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
			Debugf: func(format string, v ...interface{}) {
				fmt.Printf(format, v)
			},
		})

		if err == nil {
			if err = conn.Ping(clientCtx); err == nil {
				cancel()
				slog.Info("🥷 Connection to ClickHouse successfully")
				return &Client{
					conn: conn,
					cfg:  cfg,
				}

			}
		}

		delay := time.Duration(i+1) * time.Second
		slog.Warn("Failed to connect to ClickHouse, retrying...",
			"attempt", i+1,
			"dalay", delay,
			"error", err,
		)
		time.Sleep(delay)
	}

	cancel()
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		slog.Info("Closing ClickHouse conncetion")
		return c.conn.Close()
	}
	return nil
}

func (c Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	return c.conn.Exec(ctx, query, args...)
}

func (c *Client) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query)
}
