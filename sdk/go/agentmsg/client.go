package agentmsg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	config     *ClientConfig
	httpClient *http.Client
}

type ClientConfig struct {
	APIKey       string
	AgentID      uuid.UUID
	TenantID     uuid.UUID
	BaseURL      string
	WSURL        string
	Timeout      time.Duration
	OnMessage    func(*Message)
	OnError      func(error)
	OnConnect    func()
	OnDisconnect func()
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.agentmsg.cloud"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: config.Timeout},
	}, nil
}

func (c *Client) Connect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	if c.config.OnConnect != nil {
		c.config.OnConnect()
	}
	return nil
}

func (c *Client) Disconnect() error {
	if c.config.OnDisconnect != nil {
		c.config.OnDisconnect()
	}
	return nil
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
		return decodeAPIError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(agent)
}

func (c *Client) UpdateAgentCapabilities(ctx context.Context, capabilities Capabilities) (*Agent, error) {
	body, err := json.Marshal(map[string]any{
		"capabilities": capabilities,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.config.BaseURL+"/api/v1/agents/"+c.config.AgentID.String(), bytes.NewReader(body))
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

	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}

	var agent Agent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, err
	}
	return &agent, nil
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
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
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
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	return agents, nil
}

func (c *Client) SendMessage(ctx context.Context, msg *Message) (*SendResult, error) {
	if msg == nil {
		return nil, ErrInvalidConfig
	}
	msg.SenderID = c.config.AgentID
	msg.TenantID = c.config.TenantID
	if msg.ConversationID == uuid.Nil {
		msg.ConversationID = uuid.New()
	}

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
		return nil, decodeAPIError(resp)
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
	if resp.StatusCode != http.StatusCreated {
		return nil, decodeAPIError(resp)
	}

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
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
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
		return decodeAPIError(resp)
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
		return decodeAPIError(resp)
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
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}

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
		return decodeAPIError(resp)
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
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}

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
	if resp.StatusCode != http.StatusOK {
		return decodeAPIError(resp)
	}

	return nil
}

type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	var envelope apiErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		switch resp.StatusCode {
		case http.StatusNotFound:
			if envelope.Error.Code == "agent_not_found" {
				return ErrAgentNotFound
			}
			if envelope.Error.Code == "message_not_found" {
				return ErrMessageNotFound
			}
		}
		return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
	}

	return fmt.Errorf("request failed with status: %d", resp.StatusCode)
}
