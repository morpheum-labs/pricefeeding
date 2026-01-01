package pyth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient represents a WebSocket client for Pyth Network price feeds
type WebSocketClient struct {
	url            string
	conn           *websocket.Conn
	connMutex      sync.RWMutex
	connected      bool
	reconnectDelay time.Duration
	maxReconnects  int
	reconnectCount int

	// Message handlers
	priceUpdateHandler func(*PriceFeed)
	errorHandler       func(error)

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Subscribed price feed IDs
	subscribedIDs []HexString
	subscribedMu  sync.RWMutex

	// Connection headers
	headers map[string]string
}

// WebSocketConfig represents configuration for the WebSocket client
type WebSocketConfig struct {
	URL            string
	ReconnectDelay time.Duration
	MaxReconnects  int
	Headers        map[string]string
}

// DefaultWebSocketConfig returns a default WebSocket configuration
func DefaultWebSocketConfig(url string) *WebSocketConfig {
	return &WebSocketConfig{
		URL:            url,
		ReconnectDelay: 5 * time.Second,
		MaxReconnects:  10,
		Headers:        make(map[string]string),
	}
}

// NewWebSocketClient creates a new WebSocket client for Pyth Network
func NewWebSocketClient(config *WebSocketConfig) *WebSocketClient {
	if config == nil {
		config = DefaultWebSocketConfig("wss://hermes.pyth.network/ws")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClient{
		url:            config.URL,
		reconnectDelay: config.ReconnectDelay,
		maxReconnects:  config.MaxReconnects,
		ctx:            ctx,
		cancel:         cancel,
		headers:        config.Headers,
		subscribedIDs:  make([]HexString, 0),
	}
}

// Connect establishes a WebSocket connection to Pyth Network
func (ws *WebSocketClient) Connect() error {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	dialer := websocket.DefaultDialer

	// Convert headers map to http.Header
	header := make(http.Header)
	for key, value := range ws.headers {
		header.Set(key, value)
	}

	conn, _, err := dialer.Dial(ws.url, header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	ws.conn = conn
	ws.connected = true
	ws.reconnectCount = 0

	return nil
}

// Disconnect closes the WebSocket connection
func (ws *WebSocketClient) Disconnect() error {
	ws.cancel()

	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	if ws.conn != nil {
		err := ws.conn.Close()
		ws.conn = nil
		ws.connected = false
		return err
	}

	return nil
}

// IsConnected returns whether the WebSocket is currently connected
func (ws *WebSocketClient) IsConnected() bool {
	ws.connMutex.RLock()
	defer ws.connMutex.RUnlock()
	return ws.connected && ws.conn != nil
}

// Subscribe subscribes to price feed updates for the given price IDs
func (ws *WebSocketClient) Subscribe(priceIDs []HexString) error {
	if !ws.IsConnected() {
		return fmt.Errorf("websocket not connected")
	}

	ws.subscribedMu.Lock()
	ws.subscribedIDs = priceIDs
	ws.subscribedMu.Unlock()

	// Build subscription message
	subscribeMsg := map[string]interface{}{
		"type": "subscribe",
		"ids":  priceIDs,
	}

	ws.connMutex.RLock()
	conn := ws.conn
	ws.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to send subscription message: %w", err)
	}

	return nil
}

// Unsubscribe unsubscribes from all price feed updates
func (ws *WebSocketClient) Unsubscribe() error {
	if !ws.IsConnected() {
		return fmt.Errorf("websocket not connected")
	}

	ws.subscribedMu.Lock()
	ws.subscribedIDs = make([]HexString, 0)
	ws.subscribedMu.Unlock()

	unsubscribeMsg := map[string]interface{}{
		"type": "unsubscribe",
	}

	ws.connMutex.RLock()
	conn := ws.conn
	ws.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	if err := conn.WriteJSON(unsubscribeMsg); err != nil {
		return fmt.Errorf("failed to send unsubscribe message: %w", err)
	}

	return nil
}

// OnPriceUpdate sets the handler for price update messages
func (ws *WebSocketClient) OnPriceUpdate(handler func(*PriceFeed)) {
	ws.priceUpdateHandler = handler
}

// OnError sets the handler for error messages
func (ws *WebSocketClient) OnError(handler func(error)) {
	ws.errorHandler = handler
}

// Start begins listening for WebSocket messages
func (ws *WebSocketClient) Start() error {
	if !ws.IsConnected() {
		if err := ws.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}

	// Resubscribe if we have subscribed IDs
	ws.subscribedMu.RLock()
	subscribedIDs := ws.subscribedIDs
	ws.subscribedMu.RUnlock()

	if len(subscribedIDs) > 0 {
		if err := ws.Subscribe(subscribedIDs); err != nil {
			return fmt.Errorf("failed to resubscribe: %w", err)
		}
	}

	// Start message reading loop
	go ws.readMessages()

	return nil
}

// readMessages reads messages from the WebSocket connection
func (ws *WebSocketClient) readMessages() {
	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
		}

		ws.connMutex.RLock()
		conn := ws.conn
		ws.connMutex.RUnlock()

		if conn == nil {
			// Connection lost, attempt to reconnect
			ws.handleReconnect()
			continue
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Read message
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.handleError(fmt.Errorf("websocket read error: %w", err))
			}
			// Connection lost, attempt to reconnect
			ws.handleReconnect()
			continue
		}

		// Process message
		ws.processMessage(msg)
	}
}

// processMessage processes incoming WebSocket messages
func (ws *WebSocketClient) processMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "price_update":
		ws.handlePriceUpdate(msg)
	case "error":
		if errorMsg, ok := msg["error"].(string); ok {
			ws.handleError(fmt.Errorf("websocket error: %s", errorMsg))
		}
	default:
		// Unknown message type, ignore
	}
}

// handlePriceUpdate processes price update messages
func (ws *WebSocketClient) handlePriceUpdate(msg map[string]interface{}) {
	// Extract price_feed data (matches PythWebSocketMessage format)
	priceFeedData, ok := msg["price_feed"].(map[string]interface{})
	if !ok {
		// Try alternative format with "data" field (some WebSocket implementations use this)
		if data, ok := msg["data"].(map[string]interface{}); ok {
			priceFeedData = data
		} else if parsed, ok := msg["parsed"].([]interface{}); ok && len(parsed) > 0 {
			// Try parsed array format
			if feedMap, ok := parsed[0].(map[string]interface{}); ok {
				priceFeedData = feedMap
			}
		}
	}

	if priceFeedData == nil {
		return
	}

	// Convert to PriceFeed struct
	priceFeed := ws.parsePriceFeed(priceFeedData)
	if priceFeed != nil && ws.priceUpdateHandler != nil {
		ws.priceUpdateHandler(priceFeed)
	}
}

// parsePriceFeed parses price feed data from a map into a PriceFeed struct
func (ws *WebSocketClient) parsePriceFeed(data map[string]interface{}) *PriceFeed {
	// Marshal and unmarshal to convert map to struct
	jsonData, err := json.Marshal(data)
	if err != nil {
		ws.handleError(fmt.Errorf("failed to marshal price feed data: %w", err))
		return nil
	}

	var priceFeed PriceFeed
	if err := json.Unmarshal(jsonData, &priceFeed); err != nil {
		ws.handleError(fmt.Errorf("failed to unmarshal price feed: %w", err))
		return nil
	}

	return &priceFeed
}

// handleReconnect attempts to reconnect to the WebSocket
func (ws *WebSocketClient) handleReconnect() {
	ws.connMutex.Lock()
	ws.connected = false
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
	ws.connMutex.Unlock()

	if ws.reconnectCount >= ws.maxReconnects {
		ws.handleError(fmt.Errorf("max reconnection attempts (%d) reached", ws.maxReconnects))
		return
	}

	ws.reconnectCount++

	// Exponential backoff
	delay := ws.reconnectDelay * time.Duration(ws.reconnectCount)
	time.Sleep(delay)

	// Attempt to reconnect
	if err := ws.Connect(); err != nil {
		ws.handleError(fmt.Errorf("reconnection attempt %d failed: %w", ws.reconnectCount, err))
		// Continue trying
		go ws.handleReconnect()
		return
	}

	// Reconnection successful, resubscribe
	ws.subscribedMu.RLock()
	subscribedIDs := ws.subscribedIDs
	ws.subscribedMu.RUnlock()

	if len(subscribedIDs) > 0 {
		if err := ws.Subscribe(subscribedIDs); err != nil {
			ws.handleError(fmt.Errorf("failed to resubscribe after reconnect: %w", err))
		}
	}

	// Reset reconnect count on successful connection
	ws.reconnectCount = 0

	// Restart message reading
	go ws.readMessages()
}

// handleError calls the error handler if set
func (ws *WebSocketClient) handleError(err error) {
	if ws.errorHandler != nil {
		ws.errorHandler(err)
	}
}

// GetSubscribedIDs returns the currently subscribed price feed IDs
func (ws *WebSocketClient) GetSubscribedIDs() []HexString {
	ws.subscribedMu.RLock()
	defer ws.subscribedMu.RUnlock()

	ids := make([]HexString, len(ws.subscribedIDs))
	copy(ids, ws.subscribedIDs)
	return ids
}

// SetReadDeadline sets the read deadline for the WebSocket connection
func (ws *WebSocketClient) SetReadDeadline(t time.Time) error {
	ws.connMutex.RLock()
	defer ws.connMutex.RUnlock()

	if ws.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return ws.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline for the WebSocket connection
func (ws *WebSocketClient) SetWriteDeadline(t time.Time) error {
	ws.connMutex.RLock()
	defer ws.connMutex.RUnlock()

	if ws.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return ws.conn.SetWriteDeadline(t)
}

// SetPongHandler sets the handler for pong messages
func (ws *WebSocketClient) SetPongHandler(handler func(string) error) {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	if ws.conn != nil {
		ws.conn.SetPongHandler(handler)
	}
}

// WriteJSON writes a JSON message to the WebSocket connection
func (ws *WebSocketClient) WriteJSON(v interface{}) error {
	ws.connMutex.RLock()
	defer ws.connMutex.RUnlock()

	if ws.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return ws.conn.WriteJSON(v)
}

// ReadJSON reads a JSON message from the WebSocket connection
func (ws *WebSocketClient) ReadJSON(v interface{}) error {
	ws.connMutex.RLock()
	defer ws.connMutex.RUnlock()

	if ws.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return ws.conn.ReadJSON(v)
}
