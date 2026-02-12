package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Socket/connsock"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Socket/lib/getenv"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Socket/svr"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	wg := new(sync.WaitGroup)
	ctx, cancel := context.WithCancel(context.Background())

	connManager := connsock.NewConnectionManager(ctx, wg)

	wg.Add(1)
	go svr.StartServer(wg, connManager, ctx)

	slog.Info("🚀 Server started", "port", getenv.GetString("PORT", ":5052"))

	<-c
	cancel()
	slog.Info("👾 Received Interruption signal")
	connManager.CloseAll()
	slog.Info("⏲️ Wait for finishing all the goroutines...")
	wg.Wait()
	slog.Info("🏁 It is over 😢")
}
