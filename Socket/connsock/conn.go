package connsock

import (
	"context"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	EvTypeAggTrade = "@aggTrade"
)

type socketProducer struct {
	outputChan chan []byte

	urlConnection string
	readMsgError  chan error
	conn          *websocket.Conn
	mu            sync.RWMutex

	readMsgCancel context.CancelFunc
	readMsgWg     sync.WaitGroup

	reconnecting bool
	reconnectMu  sync.Mutex
}

func NewSocketProduecer(outChan chan []byte, url string, errChan chan error) *socketProducer {
	return &socketProducer{
		outputChan:    outChan,
		urlConnection: url,
		readMsgError:  errChan,
	}
}

func (sp *socketProducer) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	reconnectTicker := time.NewTicker(23 * time.Hour)
	defer reconnectTicker.Stop()

	conn, err := sp.connect()
	if err != nil || conn == nil {
		conn, err := sp.reconnect(ctx)
		if err != nil || conn == nil {
			return
		}
	}

	sp.mu.Lock()
	sp.conn = conn
	sp.mu.Unlock()

	defer func() {

		if sp.readMsgCancel != nil {
			sp.readMsgCancel()
		}
		sp.readMsgWg.Wait()

		sp.mu.Lock()
		if sp.conn != nil {
			sp.conn.Close()
		}
		sp.mu.Unlock()
	}()

	sp.sendPong(conn)
	sp.startReadMessage(ctx, conn)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got interrupting singal, stop receiving binance api")
			return
		case <-reconnectTicker.C:
			sp.doReconnect(ctx)

		case err := <-sp.readMsgError:
			sp.reconnectMu.Lock()
			isReconnecting := sp.reconnecting
			sp.reconnectMu.Unlock()

			if isReconnecting {
				slog.Info("Ignoring error from readMessage during reconnection", "error", err)
				continue
			}

			slog.Warn("⚠️ Connection was broken, try to connect again", "error", err)
			sp.doReconnect(ctx)
		}
	}
}

func (sp *socketProducer) startReadMessage(parentCtx context.Context, conn *websocket.Conn) {
	readCtx, cancel := context.WithCancel(parentCtx)

	sp.mu.Lock()
	sp.readMsgCancel = cancel
	sp.mu.Unlock()

	sp.readMsgWg.Add(1)

	go sp.readMessage(readCtx, conn)
}

func (sp *socketProducer) stopReadMessage() {
	sp.mu.Lock()
	cancel := sp.readMsgCancel
	sp.mu.Unlock()

	if cancel != nil {
		cancel()
		sp.readMsgWg.Wait()

		slog.Info("Old goroutine readMessage stopped")
	}
}

func (sp *socketProducer) doReconnect(ctx context.Context) {
	sp.reconnectMu.Lock()
	sp.reconnecting = true
	sp.reconnectMu.Unlock()

	defer func() {
		sp.reconnectMu.Lock()
		sp.reconnecting = false
		sp.reconnectMu.Unlock()
	}()

	sp.stopReadMessage()

	sp.mu.Lock()
	if sp.conn != nil {
		sp.conn.Close()
		sp.conn = nil
	}
	sp.mu.Unlock()

	conn, err := sp.reconnect(ctx)
	if err != nil || conn == nil {
		slog.Error("Failed to reconnect")
		return
	}

	sp.mu.Lock()
	sp.conn = conn
	sp.mu.Unlock()

	sp.sendPong(conn)
	sp.startReadMessage(ctx, conn)

	slog.Info("✅ Reconnection completed successfully")
}

func (sp *socketProducer) readMessage(ctx context.Context, conn *websocket.Conn) {
	slog.Info("📖 Entered function 'readMessage'")

	defer sp.readMsgWg.Done()

	if sp.conn == nil {
		slog.Error("'readMessage' got nil connection")
		return
	}

	go func() {
		<-ctx.Done()
		conn.SetReadDeadline(time.Now())
	}()

	for {
		_, msg, err := conn.ReadMessage()

		if err != nil {
			select {
			case <-ctx.Done():
				slog.Info("'readMessage' stopped due to context cancellation ")
				return
			default:
				sp.readMsgError <- err
				return
			}
		}

		select {
		case <-ctx.Done():
			slog.Info("'readMessage' stopped due to context cancellation ")
			return
		default:
			sp.outputChan <- msg
		}
	}
}

func (sp *socketProducer) reconnect(ctx context.Context) (*websocket.Conn, error) {
	slog.Info("🔄 Entered function 'reconnect'")

	var conn *websocket.Conn
	var err error

	const maxRetries = 5
	var attempt int

	for ; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			slog.Info("Got interrupting singal during reconnect")
			return nil, ctx.Err()
		default:
			delay := 100 * time.Millisecond
			maxDelay := 5 * time.Second
			gen := rand.New(rand.NewSource(time.Now().UnixMicro()))

			conn, err = sp.connect()

			if err == nil && conn != nil {
				return conn, nil
			}

			backoffTime := delay * time.Duration(math.Pow(2, float64(attempt)))
			backoffTime = min(backoffTime, maxDelay)

			jitter := time.Duration(gen.Int63n(int64(backoffTime)))

			time.Sleep(jitter)
		}
	}

	slog.Error("❌ Could not reconnect to binance api after all the retries")
	return nil, err
}

func (sp *socketProducer) connect() (*websocket.Conn, error) {
	currentURL := sp.currentURL()
	conn, resp, err := websocket.DefaultDialer.Dial(currentURL, nil)
	if err == nil && conn != nil {
		return conn, nil
	}
	sp.logHandshakeFailure(err, resp, currentURL)

	fallbackURL, ok := fallbackFor451(currentURL, resp)
	if !ok {
		return nil, err
	}

	slog.Warn("Switching websocket endpoint after legal restriction response",
		"from", currentURL,
		"to", fallbackURL,
	)
	sp.setURL(fallbackURL)

	conn, resp, err = websocket.DefaultDialer.Dial(fallbackURL, nil)
	if err == nil && conn != nil {
		return conn, nil
	}
	sp.logHandshakeFailure(err, resp, fallbackURL)
	return nil, err
}

func (sp *socketProducer) currentURL() string {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.urlConnection
}

func (sp *socketProducer) setURL(url string) {
	sp.mu.Lock()
	sp.urlConnection = url
	sp.mu.Unlock()
}

func (sp *socketProducer) logHandshakeFailure(err error, resp *http.Response, url string) {
	if resp == nil {
		slog.Error("❌ Could not connect to binance",
			"error", err,
			"url", url,
		)
		return
	}
	bodyText := ""
	if resp.Body != nil {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		if readErr == nil {
			bodyText = strings.TrimSpace(string(body))
		}
	}

	slog.Error("❌ WebSocket handshake failed",
		"error", err,
		"url", url,
		"status", resp.Status,
		"status_code", resp.StatusCode,
		"body", bodyText,
	)
}

func fallbackFor451(currentURL string, resp *http.Response) (string, bool) {
	if resp == nil || resp.StatusCode != http.StatusUnavailableForLegalReasons {
		return "", false
	}

	switch {
	case strings.Contains(currentURL, "stream.binance.com:443"):
		return strings.Replace(currentURL, "stream.binance.com:443", "stream.binance.us:9443", 1), true
	case strings.Contains(currentURL, "stream.binance.com:9443"):
		return strings.Replace(currentURL, "stream.binance.com:9443", "stream.binance.us:9443", 1), true
	case strings.Contains(currentURL, "stream.binance.com"):
		return strings.Replace(currentURL, "stream.binance.com", "stream.binance.us", 1), true
	default:
		return "", false
	}
}

func (sp *socketProducer) sendPong(conn *websocket.Conn) {
	slog.Info("🎾 Setting up Ping handler")

	if conn == nil {
		slog.Error("'sendPong' got nil connection")
		return
	}

	conn.SetPingHandler(func(appData string) error {
		err := conn.WriteControl(
			websocket.PongMessage,
			[]byte(appData),
			time.Now().Add(3*time.Second),
		)

		if err != nil {
			slog.Error("❌ Could not send Pong to binance", "error", err)
			return err
		}

		slog.Info("✅ Send Pong to binance successfully")
		return nil
	})
}
