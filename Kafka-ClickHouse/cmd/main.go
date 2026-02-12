package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/internal/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/adapters/clkhouse"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Kafka-ClickHouse/adapters/kaffka"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := new(sync.WaitGroup)

	clickHouseCfg := clkhouse.LoadClickHouseConfig()
	kafkaCfg := kaffka.LoadKafkaConfig()

	chClient := clkhouse.NewClient(ctx, clickHouseCfg)
	if chClient == nil {
		slog.Error("Failed to create ClickHouse client. Exiting.")
		os.Exit(1)
	}
	defer chClient.Close()

	repo := repository.NewRepository(chClient, clickHouseCfg)

	if err := repo.CreateTable(ctx); err != nil {
		slog.Error("Failed to create table", "error", err)
		os.Exit(1)
	}

	kafkaMsgs := make(chan models.KafkaMsg, 500)

	cons := kaffka.NewConsumer(ctx, kafkaCfg)

	wg.Add(2)
	go cons.Start(ctx, wg, kafkaMsgs)
	go repo.BatchInsert(ctx, wg, kafkaMsgs)

	<-c
	cancel()
	slog.Info("👾 Received shutdown signal")
	slog.Info("⏲️  Waiting for goroutines to finish...")
	wg.Wait()
	slog.Info("🏁 Shutdown complete")
}
