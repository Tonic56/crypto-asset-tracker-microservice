package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
)

type MarketTicker struct {
	MessageID     string          `db:"message_id"`
	EventType     string          `db:"event_type"`
	EventTime     time.Time       `db:"event_time"`
	ReceiveTime   time.Time       `db:"receive_time"`
	Symbol        string          `db:"symbol"`
	ClosePrice    decimal.Decimal `db:"close_price"`
	OpenPrice     decimal.Decimal `db:"open_price"`
	HighPrice     decimal.Decimal `db:"high_price"`
	LowPrice      decimal.Decimal `db:"low_price"`
	ChangePrice   decimal.Decimal `db:"change_price"`
	ChangePercent decimal.Decimal `db:"change_percent"`
}

func fetchData(conn driver.Conn) ([]MarketTicker, error) {
	ctx := context.Background()
	rows, err := conn.Query(
		ctx,
		"SELECT message_id, event_type, event_time, receive_time, symbol, close_price, open_price, high_price, low_price, change_price, change_percent FROM crypto.market_tickers ORDER BY event_time DESC LIMIT 100",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickers []MarketTicker
	for rows.Next() {
		var ticker MarketTicker
		if err := rows.Scan(
			&ticker.MessageID,
			&ticker.EventType,
			&ticker.EventTime,
			&ticker.ReceiveTime,
			&ticker.Symbol,
			&ticker.ClosePrice,
			&ticker.OpenPrice,
			&ticker.HighPrice,
			&ticker.LowPrice,
			&ticker.ChangePrice,
			&ticker.ChangePercent,
		); err != nil {
			return nil, err
		}
		tickers = append(tickers, ticker)
	}
	return tickers, nil
}

func main() {
	conn, err := connect()
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tickers, err := fetchData(conn)
		if err != nil {
			http.Error(
				w,
				fmt.Sprintf("Failed to fetch data: %v", err),
				http.StatusInternalServerError,
			)
			return
		}

		var sb strings.Builder
		sb.WriteString(
			"<html><head><title>ClickHouse Dashboard</title><style>body{font-family: sans-serif;} table{border-collapse: collapse; width: 100%;} th, td{border: 1px solid #ddd; padding: 8px;} th{background-color: #f2f2f2;}</style></head><body>",
		)
		sb.WriteString("<h1>Latest Market Tickers</h1>")
		sb.WriteString(
			"<table><tr><th>Symbol</th><th>Event Time</th><th>Close Price</th><th>Change Percent</th></tr>",
		)
		for _, t := range tickers {
			sb.WriteString(
				fmt.Sprintf(
					"<tr><td>%s</td><td>%s</td><td>%s</td><td>%s%%</td></tr>",
					t.Symbol,
					t.EventTime.Format(time.RFC3339),
					t.ClosePrice.String(),
					t.ChangePercent.String(),
				),
			)
			fmt.Println(t)
		}
		sb.WriteString("</table></body></html>")

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, sb.String())
	})

	log.Println("Starting ClickHouse Dashboard server on :8083")
	if err := http.ListenAndServe(":8083", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func connect() (driver.Conn, error) {
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chPass := os.Getenv("CLICKHOUSE_PASSWORD")

	var (
		ctx       = context.Background()
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{chAddr},
			Auth: clickhouse.Auth{
				Database: "crypto",
				Username: "default",
				Password: chPass,
			},
			DialTimeout: 10 * time.Second,
			Compression: &clickhouse.Compression{
				Method: clickhouse.CompressionLZ4,
			},
			Settings: clickhouse.Settings{
				"max_execution_time": 60,
			},
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		})
	)

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf(
				"Exception [%d] %s \n%s\n",
				exception.Code,
				exception.Message,
				exception.StackTrace,
			)
		}
		return nil, err
	}
	log.Println("Successfully connected to ClickHouse")
	return conn, nil
}
