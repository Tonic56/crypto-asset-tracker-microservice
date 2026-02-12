package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/storage/redis"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

type Client struct {
	Manager *Manager
	Conn    *websocket.Conn
	UserID  uuid.UUID
	Profile *models.User
	Send    chan []byte
	Prices  map[string]decimal.Decimal
	mu      sync.RWMutex
}

type Manager struct {
	clients         map[uuid.UUID]*Client
	mu              sync.RWMutex
	register        chan *Client
	unregister      chan *Client
	log             *slog.Logger
	subscriber      *redis.Subscriber
	coinsService    service.CoinsService
	activeRedisSub  map[string]struct{}
	coinSubscribers map[string]map[uuid.UUID]bool
	httpClient      *http.Client
}

func NewManager(log *slog.Logger, subscriber *redis.Subscriber, coinsService service.CoinsService) *Manager {
	return &Manager{
		clients:         make(map[uuid.UUID]*Client),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		log:             log,
		subscriber:      subscriber,
		coinsService:    coinsService,
		activeRedisSub:  make(map[string]struct{}),
		coinSubscribers: make(map[string]map[uuid.UUID]bool),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *Manager) Run(ctx context.Context) {
	go m.listenToRedis(ctx)

	for {
		select {
		case <-ctx.Done():
			m.log.Info("Manager run loop stopping...")
			return
		case client := <-m.register:
			m.registerClient(client)
		case client := <-m.unregister:
			m.unregisterClient(client)
		}
	}
}

func (m *Manager) listenToRedis(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			m.log.Info("Redis listener stopping...")
			return
		case msg, ok := <-m.subscriber.Messages:
			if !ok {
				m.log.Warn("manager redis subscriber channel closed")
				return
			}
			m.processRedisMessage(msg)
		}
	}
}

func (m *Manager) Register(client *Client) {
	m.register <- client
}

func (m *Manager) Unregister(client *Client) {
	m.unregister <- client
}

func (m *Manager) registerClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if oldClient, exists := m.clients[client.UserID]; exists {
		m.log.Warn("client re-registering, closing old connection", "userID", client.UserID)
		close(oldClient.Send)
		oldClient.Conn.Close()
	}

	client.Prices = make(map[string]decimal.Decimal)
	m.clients[client.UserID] = client
	m.log.Info("new client registered", "userID", client.UserID)

	for _, coin := range client.Profile.Coins {
		m.followCoin(client.UserID, coin.Symbol)
	}
}

func (m *Manager) unregisterClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.clients[client.UserID]; ok {
		delete(m.clients, client.UserID)
		m.unfollowAllCoins(client.UserID)
		m.log.Info("client unregistered", "userID", client.UserID)
	}
}

func (m *Manager) followCoin(userID uuid.UUID, symbol string) {
	if _, ok := m.coinSubscribers[symbol]; !ok {
		m.coinSubscribers[symbol] = make(map[uuid.UUID]bool)
	}
	m.coinSubscribers[symbol][userID] = true

	if _, ok := m.activeRedisSub[symbol]; !ok {
		m.log.Info("first subscriber for symbol, telling aggregator to start stream", "symbol", symbol, "userID", userID)
		go m.notifyAggregator(userID.String(), symbol, http.MethodGet)

		if err := m.subscriber.Subscribe(context.Background(), symbol); err != nil {
			m.log.Error("manager: could not subscribe to coin stream", "coin", symbol, "error", err)
			return
		}
		m.activeRedisSub[symbol] = struct{}{}
	}
}

func (m *Manager) unfollowAllCoins(userID uuid.UUID) {
	for symbol, users := range m.coinSubscribers {
		if _, ok := users[userID]; ok {
			delete(users, userID)
			m.log.Info("user unfollowed coin", "userID", userID, "symbol", symbol)
		}

		if len(users) == 0 {
			m.log.Info("no subscribers left, telling aggregator to stop stream", "symbol", symbol)
			go m.notifyAggregator(userID.String(), symbol, http.MethodDelete)

			delete(m.coinSubscribers, symbol)
			delete(m.activeRedisSub, symbol)
			if err := m.subscriber.Unsubscribe(context.Background(), symbol); err != nil {
				m.log.Error("manager: failed to unsubscribe from redis", "symbol", symbol, "error", err)
			}
		}
	}
}

func (m *Manager) processRedisMessage(msg redis.Message) {
	var priceUpdate models.PriceUpdate
	if err := json.Unmarshal([]byte(msg.Payload), &priceUpdate); err != nil {
		m.log.Error("failed to parse price update from redis", "error", err, "payload", msg.Payload)
		return
	}

	priceDecimal := decimal.NewFromFloat(priceUpdate.Price)

	m.mu.RLock()
	defer m.mu.RUnlock()

	subscribers, ok := m.coinSubscribers[priceUpdate.Symbol]
	if !ok {
		return
	}

	for userID := range subscribers {
		if client, ok := m.clients[userID]; ok {
			client.mu.Lock()

			client.Prices[priceUpdate.Symbol] = priceDecimal

			portfolio := models.PortfolioView{
				UserID:     client.UserID.String(),
				UserName:   client.Profile.Name,
				TotalValue: decimal.Zero,
				Coins:      []models.CoinView{},
			}

			for _, coin := range client.Profile.Coins {
				currentPrice, priceFound := client.Prices[coin.Symbol]
				if !priceFound {
					currentPrice = decimal.Zero
				}

				total := coin.Quantity.Mul(currentPrice)
				portfolio.Coins = append(portfolio.Coins, models.CoinView{
					Symbol:   coin.Symbol,
					Quantity: coin.Quantity,
					Price:    currentPrice,
					Total:    total,
				})
				portfolio.TotalValue = portfolio.TotalValue.Add(total)
			}

			jsonData, err := json.Marshal(portfolio)
			if err != nil {
				m.log.Error("failed to marshal portfolio view", "error", err, "userID", userID)
				client.mu.Unlock()
				continue
			}

			select {
			case client.Send <- jsonData:
			default:
				m.log.Warn("client send channel is full, dropping message", "userID", userID)
			}
			client.mu.Unlock()
		}
	}
}

func (c *Client) Writer() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.Manager.log.Warn("failed to write message to client", "userID", c.UserID)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Reader() {
	defer func() {
		c.Manager.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })

	for {
		if _, _, err := c.Conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Manager.log.Warn("unexpected close error", "userID", c.UserID, "error", err)
			}
			break
		}
	}
}

func (m *Manager) notifyAggregator(userID, symbol, method string) {
	aggregatorURL := fmt.Sprintf(
		"http://aggregator-service:8088/coin?symbol=%s&id=%s",
		url.QueryEscape(symbol),
		url.QueryEscape(userID),
	)

	req, err := http.NewRequest(method, aggregatorURL, nil)
	if err != nil {
		m.log.Error("manager: failed to create request for aggregator", "error", err)
		return
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.log.Error("manager: failed to notify aggregator", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		m.log.Warn("manager: aggregator returned non-200 status", "status", resp.Status)
	} else {
		m.log.Info("manager: successfully notified aggregator", "method", method, "symbol", symbol)
	}
}