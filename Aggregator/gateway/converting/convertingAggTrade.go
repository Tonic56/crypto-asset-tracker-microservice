package converting

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/models"
)

func ConvertRawToSS(
	ctx context.Context,
	wg *sync.WaitGroup,
	inChan chan []byte,
	outChan chan models.SecondStat,
) {
	defer wg.Done()
	defer close(outChan)

	wgWorker := new(sync.WaitGroup)
	aggTrChan := make(chan models.AggTrade, 100)

	wgWorker.Add(1)
	go receiveSecondStat(ctx, wgWorker, aggTrChan, outChan)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got Interruption signal, stopping to converting messages from stream")
			wgWorker.Wait()
			return
		case msg, ok := <-inChan:
			if !ok {
				return
			}
			var aggTrade models.AggTrade
			if err := json.Unmarshal(msg, &aggTrade); err != nil {
				slog.Error("Could not parse bytes into aggTrade struct", "error", err)
				continue
			}

			select {
			case <-ctx.Done():
				slog.Info("Got Interruption signal, stopping to converting messages from stream")
				wgWorker.Wait()
				return
			case aggTrChan <- aggTrade:
			}
		}
	}
}

func receiveSecondStat(
	ctx context.Context,
	wg *sync.WaitGroup,
	inChan chan models.AggTrade,
	outChan chan models.SecondStat,
) {
	defer wg.Done()

	latestPrices := make(map[string]float64)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info(
				"Got Interruption signal, stopping to receiving messages from aggTrade chan",
			)
			return
		case msg, ok := <-inChan:
			if !ok {
				return
			}
			symbol := strings.ToLower(msg.Symbol)
			latestPrices[symbol] = msg.PriceFloat()
		case <-ticker.C:
			for symbol, price := range latestPrices {
				secondStat := models.SecondStat{
					Symbol: symbol,
					Price:  price,
				}
				select {
				case <-ctx.Done():
					slog.Info(
						"Got Interruption signal, stopping to receiving messages from aggTrade chan",
					)
					return
				case outChan <- secondStat:
				default:
					slog.Warn("SecondStat channel is full, dropping message", "symbol", symbol)
				}
			}
		}
	}
}
