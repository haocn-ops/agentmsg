package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID        `json:"id" db:"id"`
	ConversationID uuid.UUID        `json:"conversationId" db:"conversation_id"`
	MessageType   MessageType      `json:"messageType" db:"message_type"`
	SenderID       uuid.UUID        `json:"senderId" db:"sender_id"`
	RecipientIDs   []uuid.UUID      `json:"recipientIds" db:"-"`
	RecipientStr   string           `json:"-" db:"recipient_ids"`
	Content        []byte           `json:"content" db:"content"`
	ContentSize    int              `json:"contentSize" db:"content_size"`
	ContentType    string           `json:"contentType" db:"content_type"`
	Metadata       MessageMetadata  `json:"metadata" db:"metadata"`
	DeliveryGuarantee DeliveryGuarantee `json:"deliveryGuarantee" db:"delivery_guarantee"`
	Status         MessageStatus     `json:"status" db:"status"`
	TaskContext    *TaskContext      `json:"taskContext,omitempty" db:"task_context"`
	TraceID        string           `json:"traceId" db:"trace_id"`
	TenantID       uuid.UUID        `json:"tenantId" db:"tenant_id"`
	CreatedAt      time.Time        `json:"createdAt" db:"created_at"`
	ExpiresAt      *time.Time       `json:"expiresAt,omitempty" db:"expires_at"`
	ProcessedAt    *time.Time       `json:"processedAt,omitempty" db:"processed_at"`
}

func (m *Message) ScanRecipients() error {
	if m.RecipientStr != "" {
		return json.Unmarshal([]byte(m.RecipientStr), &m.RecipientIDs)
	}
	m.RecipientIDs = []uuid.UUID{}
	return nil
}

func (m *Message) SetRecipients() error {
	if len(m.RecipientIDs) == 0 {
		m.RecipientStr = "[]"
		return nil
	}

	data, err := json.Marshal(m.RecipientIDs)
	if err != nil {
		return err
	}

	m.RecipientStr = string(data)
	return nil
}

type MessageType string

const (
	MessageTypeTaskRequest      MessageType = "task.request"
	MessageTypeTaskResponse     MessageType = "task.response"
	MessageTypeTaskStatusUpdate MessageType = "task.status-update"
	MessageTypeTaskDelegate     MessageType = "task.delegate"
	MessageTypeCapabilityQuery  MessageType = "capability.query"
	MessageTypeCapabilityAdvert MessageType = "capability.advertise"
	MessageTypeErrorReport      MessageType = "error.report"
	MessageTypeHeartbeat        MessageType = "heartbeat"
	MessageTypeGeneric          MessageType = "generic"
)

type DeliveryGuarantee string

const (
	DeliveryAtMostOnce  DeliveryGuarantee = "at_most_once"
	DeliveryAtLeastOnce DeliveryGuarantee = "at_least_once"
	DeliveryExactlyOnce DeliveryGuarantee = "exactly_once"
)

type MessageStatus string

const (
	MessageStatusPending    MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusProcessed MessageStatus = "processed"
	MessageStatusFailed   MessageStatus = "failed"
	MessageStatusDeadLetter MessageStatus = "dead_letter"
)

type DeadLetterStatus string

const (
	DeadLetterStatusPending   DeadLetterStatus = "pending"
	DeadLetterStatusProcessed DeadLetterStatus = "processed"
	DeadLetterStatusExhausted DeadLetterStatus = "exhausted"
)

type MessageMetadata struct {
	Tags         map[string]string `json:"tags,omitempty"`
	CorrelationID string           `json:"correlationId,omitempty"`
	ReplyTo      *uuid.UUID       `json:"replyTo,omitempty"`
	RoutingHints *RoutingHints     `json:"routingHints,omitempty"`
	Compression  string           `json:"compression,omitempty"`
	Encoding     string           `json:"encoding,omitempty"`
	Custom       map[string]any   `json:"custom,omitempty"`
}

func (m MessageMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *MessageMetadata) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), m)
}

type RoutingHints struct {
	PreferredAgents []string `json:"preferredAgents,omitempty"`
	ExcludedAgents  []string `json:"excludedAgents,omitempty"`
	MaxHops        int      `json:"maxHops,omitempty"`
	PriceLimit     float64  `json:"priceLimit,omitempty"`
}

type TaskContext struct {
	TaskID       uuid.UUID  `json:"taskId"`
	ParentTaskID *uuid.UUID `json:"parentTaskId,omitempty"`
	RootTaskID   *uuid.UUID `json:"rootTaskId,omitempty"`
	Priority     int        `json:"priority"`
	Deadline     *time.Time `json:"deadline,omitempty"`
	Dependencies []uuid.UUID `json:"dependencies,omitempty"`
	Blocking     bool       `json:"blocking,omitempty"`
	RetryPolicy  *RetryPolicy `json:"retryPolicy,omitempty"`
	RetryCount   int        `json:"retryCount"`
}

func (t TaskContext) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *TaskContext) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), t)
}

type RetryPolicy struct {
	MaxAttempts      int     `json:"maxAttempts"`
	InitialDelayMs   int     `json:"initialDelayMs"`
	MaxDelayMs       int     `json:"maxDelayMs"`
	BackoffMultiplier float64 `json:"backoffMultiplier"`
}

type Acknowledgement struct {
	ID          uuid.UUID `json:"id" db:"id"`
	MessageID   uuid.UUID `json:"messageId" db:"message_id"`
	AgentID     uuid.UUID `json:"agentId" db:"agent_id"`
	Status      AckStatus `json:"status" db:"status"`
	Details     string    `json:"details,omitempty" db:"details"`
	ProcessedAt *time.Time `json:"processedAt,omitempty" db:"processed_at"`
	Nonce       string    `json:"nonce" db:"nonce"`
	Signature   string    `json:"signature,omitempty" db:"signature"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

type AckStatus string

const (
	AckStatusReceived  AckStatus = "received"
	AckStatusProcessed AckStatus = "processed"
	AckStatusRejected AckStatus = "rejected"
	AckStatusFailed   AckStatus = "failed"
)

type DeadLetterEntry struct {
	ID         uuid.UUID        `json:"id" db:"id"`
	MessageID  uuid.UUID        `json:"messageId" db:"message_id"`
	Reason     string           `json:"reason" db:"reason"`
	RetryCount int              `json:"retryCount" db:"retry_count"`
	MaxRetries int              `json:"maxRetries" db:"max_retries"`
	Payload    []byte           `json:"payload" db:"payload"`
	Status     DeadLetterStatus `json:"status" db:"status"`
	CreatedAt  time.Time        `json:"createdAt" db:"created_at"`
	ProcessedAt *time.Time      `json:"processedAt,omitempty" db:"processed_at"`
}
