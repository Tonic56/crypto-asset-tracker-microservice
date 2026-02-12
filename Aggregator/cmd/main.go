package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/gateway/converting"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/gateway/strman"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/lib/getenv"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/adapters/reddis"
	"github.com/gin-gonic/gin"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/adapters/kaffka"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)

	rawMsgsChan := make(chan []byte, 300)

	rawAggTradeChan := make(chan []byte, 100)
	rawMiniTickerChan := make(chan []byte, 100)

	secondStatChan := make(chan models.SecondStat, 100)
	dailyStatChan := make(chan models.DailyStat, 500)
	kafkaMsgChan := make(chan models.KafkaMsg, 500)

	streamManager := strman.NewStreamManager()

	r := gin.Default()

	r.GET("/coin", func(c *gin.Context) {
		symbol := c.Query("symbol")
		id := c.Query("id")

		if symbol == "" || id == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "symbol and id query parameters are required",
			})
			return
		}

		started := streamManager.AddCoin(ctx, wg, symbol, id, rawMsgsChan)

		if started {
			c.JSON(http.StatusOK, gin.H{
				"status": "subscription_added",
				"symbol": symbol,
				"userID": id,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status": "already_subscribed",
				"symbol": symbol,
				"userID": id,
			})
		}
	})

	r.DELETE("/coin", func(c *gin.Context) {
		symbol := c.Query("symbol")
		id := c.Query("id")

		if symbol == "" || id == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "symbol and id query parameters are required",
			})
			return
		}

		streamManager.DeleteCoin(symbol, id)

		c.JSON(http.StatusOK, gin.H{
			"status": "subscription_removed",
			"symbol": symbol,
			"userID": id,
		})
	})

	server := http.Server{
		Addr:    getenv.GetString("SERVER_ADDR", ":8088"),
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			slog.Error("Failed to run server", "error", err)
			c <- os.Interrupt
		}
	}()

	cfgKafka := kaffka.LoadKafkaConfig()
	producer := kaffka.NewProducer(cfgKafka)

	cfgRedis := reddis.LoadRedisConfig()
	saver := reddis.NewSaver(cfgRedis)

	wg.Add(7)

	go converting.DistributeMessages(ctx, wg, rawMsgsChan, rawAggTradeChan, rawMiniTickerChan)

	go converting.ReceiveMiniTickerMessage(ctx, wg, rawMsgsChan)

	go converting.ConvertRawToArrDS(ctx, wg, rawMiniTickerChan, dailyStatChan)
	go converting.ConvertRawToSS(ctx, wg, rawAggTradeChan, secondStatChan)

	go converting.ReceiveKafkaMsg(ctx, wg, dailyStatChan, kafkaMsgChan)
	go producer.Start(ctx, wg, kafkaMsgChan)

	go saver.Start(ctx, wg, secondStatChan)

	<-c
	slog.Info("👾 Received Interruption signal")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to gracefully shutdown HTTP server", "error", err)
	} else {
		slog.Info("✅ HTTP server stopped gracefully")
	}

	slog.Info("⏲️ Wait for finishing all the goroutines...")
	wg.Wait()
	slog.Info("🏁 It is over 😢")
}
