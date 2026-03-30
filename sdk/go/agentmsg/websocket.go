package agentmsg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	config          *ClientConfig
	conn            *websocket.Conn
	mu              sync.RWMutex
	sendCh          chan []byte
	recvCh          chan *WSMessage
	doneCh          chan struct{}
	shouldReconnect bool
	maxRetries      int
	retries         int
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewWebSocketClient(config *ClientConfig) *WebSocketClient {
	return &WebSocketClient{
		config:     config,
		sendCh:     make(chan []byte, 100),
		recvCh:     make(chan *WSMessage, 100),
		doneCh:     make(chan struct{}),
		maxRetries: 5,
		retries:    0,
	}
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+c.config.APIKey)

	u := fmt.Sprintf("%s/ws?agent_id=%s&tenant_id=%s",
		c.config.WSURL,
		c.config.AgentID.String(),
		c.config.TenantID.String())

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u, header)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.conn = conn
	c.shouldReconnect = true
	c.retries = 0

	go c.readLoop()
	go c.writeLoop()

	return nil
}

func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	c.shouldReconnect = false
	c.mu.Unlock()

	close(c.doneCh)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *WebSocketClient) Send(msg *WSMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *WebSocketClient) readLoop() {
	for {
		select {
		case <-c.doneCh:
			return
		default:
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				if c.canReconnect() {
					c.reconnectLoop()
					continue
				}
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			select {
			case c.recvCh <- &msg:
			case <-c.doneCh:
				return
			}
		}
	}
}

func (c *WebSocketClient) writeLoop() {
	for {
		select {
		case <-c.doneCh:
			return
		case data := <-c.sendCh:
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

func (c *WebSocketClient) canReconnect() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.shouldReconnect && c.retries < c.maxRetries
}

func (c *WebSocketClient) reconnectLoop() {
	c.mu.Lock()
	c.retries++
	c.mu.Unlock()

	backoff := time.Duration(c.retries) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	time.Sleep(backoff)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		return
	}
}

func (c *WebSocketClient) OnMessage(handler func(*WSMessage)) {
	go func() {
		for msg := range c.recvCh {
			handler(msg)
		}
	}()
}

func (c *WebSocketClient) OnConnect(handler func()) {
}

func (c *WebSocketClient) OnDisconnect(handler func()) {
}

func (c *WebSocketClient) SendMessage(msgType string, data interface{}) error {
	rawData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.Send(&WSMessage{
		Type: msgType,
		Data: rawData,
	})
}

func SendTextMessage(ctx context.Context, conn *websocket.Conn, msgType string, data interface{}) error {
	rawData, err := json.Marshal(map[string]interface{}{
		"type": msgType,
		"data": data,
	})
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, rawData)
}

func ReadWSMessage(conn *websocket.Conn, timeout time.Duration) (*WSMessage, error) {
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)

	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

type IdempotencyKey struct {
	Key       string    `json:"key"`
	MessageID uuid.UUID `json:"messageId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}
