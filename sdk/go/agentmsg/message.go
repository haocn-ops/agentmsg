package agentmsg

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID               uuid.UUID         `json:"id"`
	ConversationID   uuid.UUID         `json:"conversationId"`
	MessageType      MessageType       `json:"messageType"`
	SenderID         uuid.UUID         `json:"senderId"`
	RecipientIDs     []uuid.UUID        `json:"recipientIds"`
	Content          []byte            `json:"content"`
	ContentSize      int               `json:"contentSize"`
	ContentType      string            `json:"contentType"`
	Metadata         MessageMetadata   `json:"metadata"`
	DeliveryGuarantee DeliveryGuarantee `json:"deliveryGuarantee"`
	Status           MessageStatus     `json:"status"`
	TaskContext      *TaskContext      `json:"taskContext,omitempty"`
	TraceID          string            `json:"traceId"`
	TenantID         uuid.UUID         `json:"tenantId"`
	CreatedAt        time.Time         `json:"createdAt"`
	ExpiresAt        *time.Time        `json:"expiresAt,omitempty"`
	ProcessedAt      *time.Time        `json:"processedAt,omitempty"`
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
	MessageStatusFailed    MessageStatus = "failed"
)

type MessageMetadata struct {
	Tags          map[string]string `json:"tags,omitempty"`
	CorrelationID string            `json:"correlationId,omitempty"`
	ReplyTo       *uuid.UUID        `json:"replyTo,omitempty"`
	RoutingHints  *RoutingHints     `json:"routingHints,omitempty"`
	Compression   string            `json:"compression,omitempty"`
	Encoding      string            `json:"encoding,omitempty"`
	Custom        map[string]any    `json:"custom,omitempty"`
}

type RoutingHints struct {
	PreferredAgents []string `json:"preferredAgents,omitempty"`
	ExcludedAgents  []string `json:"excludedAgents,omitempty"`
	MaxHops        int      `json:"maxHops,omitempty"`
	PriceLimit     float64  `json:"priceLimit,omitempty"`
}

type TaskContext struct {
	TaskID       uuid.UUID    `json:"taskId"`
	ParentTaskID *uuid.UUID   `json:"parentTaskId,omitempty"`
	RootTaskID   *uuid.UUID   `json:"rootTaskId,omitempty"`
	Priority     int          `json:"priority"`
	Deadline     *time.Time   `json:"deadline,omitempty"`
	Dependencies []uuid.UUID  `json:"dependencies,omitempty"`
	Blocking     bool         `json:"blocking,omitempty"`
	RetryPolicy  *RetryPolicy  `json:"retryPolicy,omitempty"`
	RetryCount   int          `json:"retryCount"`
}

func (t TaskContext) Value() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TaskContext) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), t)
}

type RetryPolicy struct {
	MaxAttempts       int     `json:"maxAttempts"`
	InitialDelayMs    int     `json:"initialDelayMs"`
	MaxDelayMs        int     `json:"maxDelayMs"`
	BackoffMultiplier float64 `json:"backoffMultiplier"`
}

type Acknowledgement struct {
	ID          uuid.UUID `json:"id"`
	MessageID   uuid.UUID `json:"messageId"`
	AgentID     uuid.UUID `json:"agentId"`
	Status      AckStatus `json:"status"`
	Details     string    `json:"details,omitempty"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
	Nonce       string    `json:"nonce"`
	Signature   string    `json:"signature,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AckStatus string

const (
	AckStatusReceived  AckStatus = "received"
	AckStatusProcessed AckStatus = "processed"
	AckStatusRejected  AckStatus = "rejected"
	AckStatusFailed    AckStatus = "failed"
)

type SendResult struct {
	MessageID   uuid.UUID `json:"messageId"`
	Status      string    `json:"status"`
	DeliveredAt *int64    `json:"deliveredAt,omitempty"`
}

type Subscription struct {
	ID        uuid.UUID           `json:"id"`
	AgentID   uuid.UUID           `json:"agentId"`
	TenantID  uuid.UUID           `json:"tenantId"`
	Type      SubType             `json:"type"`
	Filter    SubscriptionFilter  `json:"filter"`
	Status    SubStatus           `json:"status"`
	CreatedAt int64               `json:"createdAt"`
}

type SubType string

const (
	SubTypeDirect     SubType = "direct"
	SubTypeCapability SubType = "capability"
	SubTypeTopic      SubType = "topic"
	SubTypePattern    SubType = "pattern"
)

type SubStatus string

const (
	SubStatusActive    SubStatus = "active"
	SubStatusPaused   SubStatus = "paused"
	SubStatusCancelled SubStatus = "cancelled"
)

type SubscriptionFilter struct {
	AgentIDs         []uuid.UUID      `json:"agentIds,omitempty"`
	CapabilityTypes  []string         `json:"capabilityTypes,omitempty"`
	Topics           []string         `json:"topics,omitempty"`
	MessageTypes     []string         `json:"messageTypes,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
}