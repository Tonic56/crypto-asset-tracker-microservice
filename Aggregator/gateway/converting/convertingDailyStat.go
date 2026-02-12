package converting

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func ConvertRawToArrDS(
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChan chan []byte,
	outputChan chan models.DailyStat,
) {
	defer wg.Done()
	defer close(outputChan)

	numWorkers := 4
	workerChan := make(chan models.MiniTicker, 100)
	wgWorker := new(sync.WaitGroup)

	defer close(workerChan)

	wgWorker.Add(numWorkers)
	for range 4 {
		go ReceiveDailyStat(ctx, wgWorker, workerChan, outputChan)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got Interruption signal, stopping to converting messages from stream")
			return
		case arrBytes, ok := <-inputChan:
			if !ok {
				return
			}
			var arrMsgs []models.MiniTicker
			if err := json.Unmarshal(arrBytes, &arrMsgs); err != nil {
				slog.Error("Could not parse bytes into array of miniTickers", "error", err)
				continue
			}

			slog.Debug("Received batch", "size", len(arrMsgs))

			select {
			case <-ctx.Done():
				wgWorker.Wait()
				slog.Info("Got Interruption signal, stopping to converting messages from stream")
			default:
				for _, msg := range arrMsgs {
					select {
					case <-ctx.Done():
						wgWorker.Wait()
						slog.Info(
							"Got Interruption signal, stopping to converting messages from stream",
						)
						return
					case workerChan <- msg:
					}
				}
			}

		}
	}
}

func ReceiveDailyStat(
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChan chan models.MiniTicker,
	outputChan chan models.DailyStat,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			slog.Info(
				"Got Interruption signal, stopping to receiving messages from MiniTicker chan",
			)
			return
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			dailyStat := models.DailyStat{
				EventType:  msg.EventType,
				EventTime:  msg.EventTime,
				RecvTime:   time.Now().UnixMilli(),
				Symbol:     msg.Symbol,
				ClosePrice: msg.ClosePriceFloat(),
				OpenPrice:  msg.OpenPriceFloat(),
				HighPrice:  msg.HighPriceFloat(),
				LowPrice:   msg.LowPriceFloat(),
			}
			select {
			case <-ctx.Done():
				slog.Info(
					"Got Interruption signal, stopping to receiving messages from MiniTicker chan",
				)
				return
			default:
				outputChan <- dailyStat
			}
		}
	}
}

func ReceiveKafkaMsg(
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChan chan models.DailyStat,
	outputChan chan models.KafkaMsg,
) {
	defer wg.Done()
	defer close(outputChan)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got Interruption signal, stopping to sending messages from DailyStat chan")
			return
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			kafkaMsg := models.KafkaMsg{
				MessageID:     uuid.New().String(),
				EventType:     msg.EventType,
				EventTime:     msg.EventTime,
				RecvTime:      msg.RecvTime,
				Symbol:        msg.Symbol,
				ClosePrice:    decimal.NewFromFloat(msg.ClosePrice),
				OpenPrice:     decimal.NewFromFloat(msg.OpenPrice),
				HighPrice:     decimal.NewFromFloat(msg.HighPrice),
				LowPrice:      decimal.NewFromFloat(msg.LowPrice),
				ChangePrice:   msg.ChangeInPrice(),
				ChangePercent: msg.ChangeInPercent(),
			}
			select {
			case <-ctx.Done():
				slog.Info(
					"Got Interruption signal, stopping to sending messages from DailyStat chan",
				)
				return
			default:
				outputChan <- kafkaMsg
			}
		}
	}
}
