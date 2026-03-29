package agentmsg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	ws         *WebSocketClient
	mu         sync.RWMutex
	agents     map[uuid.UUID]*Agent
	ctx        context.Context
	cancel     context.CancelFunc
}

type ClientConfig struct {
	APIKey        string
	AgentID       uuid.UUID
	TenantID      uuid.UUID
	BaseURL       string
	WSURL         string
	Timeout       time.Duration
	OnMessage     func(*Message)
	OnError       func(error)
	OnConnect     func()
	OnDisconnect  func()
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.agentmsg.cloud"
	}
	if config.WSURL == "" {
		config.WSURL = "wss://ws.agentmsg.cloud"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: config.Timeout},
		ws:         NewWebSocketClient(config),
		agents:     make(map[uuid.UUID]*Agent),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (c *Client) Connect(ctx context.Context) error {
	if err := c.ws.Connect(ctx); err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	c.ws.OnMessage(func(msg *WSMessage) {
		c.handleWSMessage(msg)
	})

	c.ws.OnConnect(func() {
		if c.config.OnConnect != nil {
			c.config.OnConnect()
		}
	})

	c.ws.OnDisconnect(func() {
		if c.config.OnDisconnect != nil {
			c.config.OnDisconnect()
		}
	})

	return nil
}

func (c *Client) Disconnect() error {
	c.cancel()
	return c.ws.Close()
}

func (c *Client) RegisterAgent(ctx context.Context, agent *Agent) error {
	body, err := json.Marshal(agent)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/agents", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(agent)
}

func (c *Client) GetAgent(ctx context.Context, agentID uuid.UUID) (*Agent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/v1/agents/"+agentID.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrAgentNotFound
	}

	var agent Agent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, err
	}

	return &agent, nil
}

func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/v1/agents", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	return agents, nil
}

func (c *Client) SendMessage(ctx context.Context, msg *Message) (*SendResult, error) {
	msg.SenderID = c.config.AgentID

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("send failed with status: %d", resp.StatusCode)
	}

	var result SendResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) SendBatchMessages(ctx context.Context, messages []*Message) ([]SendResult, error) {
	body, err := json.Marshal(map[string]interface{}{"messages": messages})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/messages/batch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []SendResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/v1/messages/"+messageID.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMessageNotFound
	}

	var msg Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (c *Client) AcknowledgeMessage(ctx context.Context, messageID uuid.UUID, status AckStatus) error {
	body, err := json.Marshal(map[string]string{"status": string(status)})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/messages/"+messageID.String()+"/ack", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ack failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) CreateSubscription(ctx context.Context, sub *Subscription) error {
	body, err := json.Marshal(sub)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/subscriptions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("subscription creation failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/v1/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Subscriptions, nil
}

func (c *Client) DeleteSubscription(ctx context.Context, subscriptionID uuid.UUID) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.config.BaseURL+"/api/v1/subscriptions/"+subscriptionID.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete subscription failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) QueryCapabilities(ctx context.Context, capabilities []string) ([]Agent, error) {
	body, err := json.Marshal(map[string]interface{}{"capabilities": capabilities})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/discovery/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Agents []Agent `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Agents, nil
}

func (c *Client) Heartbeat(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/v1/agents/"+c.config.AgentID.String()+"/heartbeat", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) handleWSMessage(msg *WSMessage) {
	switch msg.Type {
	case "message":
		var message Message
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			if c.config.OnError != nil {
				c.config.OnError(err)
			}
			return
		}
		if c.config.OnMessage != nil {
			c.config.OnMessage(&message)
		}
	case "ack":
		var ack Ack
		if err := json.Unmarshal(msg.Data, &ack); err != nil {
			if c.config.OnError != nil {
				c.config.OnError(err)
			}
		}
	}
}