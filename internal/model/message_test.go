package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMessageCreation(t *testing.T) {
	msg := &Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   MessageTypeTaskRequest,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte(`{"task":"test"}`),
		ContentSize:   14,
		ContentType:   "application/json",
		DeliveryGuarantee: DeliveryAtLeastOnce,
		Status:        MessageStatusPending,
		TenantID:      uuid.New(),
		CreatedAt:     time.Now(),
	}

	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, MessageTypeTaskRequest, msg.MessageType)
	assert.Equal(t, MessageStatusPending, msg.Status)
}

func TestMessageMetadataSerialization(t *testing.T) {
	metadata := MessageMetadata{
		Tags: map[string]string{
			"priority": "high",
			"env":      "prod",
		},
		CorrelationID: "corr-123",
		Compression:  "gzip",
	}

	data, err := metadata.Value()
	assert.NoError(t, err)

	var result MessageMetadata
	err = result.Scan(data)
	assert.NoError(t, err)
	assert.Equal(t, "corr-123", result.CorrelationID)
	assert.Equal(t, "high", result.Tags["priority"])
}

func TestTaskContextSerialization(t *testing.T) {
	taskID := uuid.New()
	deadline := time.Now().Add(time.Hour)

	taskCtx := &TaskContext{
		TaskID:   taskID,
		Priority: 2,
		Deadline: &deadline,
		RetryPolicy: &RetryPolicy{
			MaxAttempts:      3,
			InitialDelayMs:   1000,
			MaxDelayMs:       30000,
			BackoffMultiplier: 2.0,
		},
		RetryCount: 0,
	}

	data, err := taskCtx.Value()
	assert.NoError(t, err)

	var result TaskContext
	err = result.Scan(data)
	assert.NoError(t, err)
	assert.Equal(t, taskID, result.TaskID)
	assert.Equal(t, 2, result.Priority)
	assert.Equal(t, 3, result.RetryPolicy.MaxAttempts)
}

func TestDeliveryGuaranteeTypes(t *testing.T) {
	tests := []struct {
		guarantee DeliveryGuarantee
		want       string
	}{
		{DeliveryAtMostOnce, "at_most_once"},
		{DeliveryAtLeastOnce, "at_least_once"},
		{DeliveryExactlyOnce, "exactly_once"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.guarantee))
		})
	}
}

func TestMessageStatusTransitions(t *testing.T) {
	msg := &Message{
		ID:     uuid.New(),
		Status: MessageStatusPending,
	}

	msg.Status = MessageStatusSent
	assert.Equal(t, MessageStatusSent, msg.Status)

	msg.Status = MessageStatusDelivered
	assert.Equal(t, MessageStatusDelivered, msg.Status)

	msg.Status = MessageStatusProcessed
	assert.Equal(t, MessageStatusProcessed, msg.Status)
}

func TestMessageJSONSerialization(t *testing.T) {
	msg := &Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   MessageTypeGeneric,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte("hello"),
		ContentSize:   5,
		DeliveryGuarantee: DeliveryAtLeastOnce,
		Status:        MessageStatusPending,
		TenantID:      uuid.New(),
		CreatedAt:     time.Now(),
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var result Message
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, msg.ID, result.ID)
	assert.Equal(t, msg.MessageType, result.MessageType)
}

func TestRoutingHints(t *testing.T) {
	hints := &RoutingHints{
		PreferredAgents: []string{"agent-1", "agent-2"},
		ExcludedAgents: []string{"agent-3"},
		MaxHops:       3,
		PriceLimit:     0.01,
	}

	data, err := json.Marshal(hints)
	assert.NoError(t, err)

	var result RoutingHints
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Len(t, result.PreferredAgents, 2)
	assert.Equal(t, 3, result.MaxHops)
}

func TestAcknowledgementCreation(t *testing.T) {
	ack := &Acknowledgement{
		ID:        uuid.New(),
		MessageID: uuid.New(),
		AgentID:   uuid.New(),
		Status:    AckStatusReceived,
		Nonce:     "unique-nonce-123",
		CreatedAt: time.Now(),
	}

	assert.NotEmpty(t, ack.ID)
	assert.Equal(t, AckStatusReceived, ack.Status)
}

func TestAckStatusTransitions(t *testing.T) {
	ack := &Acknowledgement{
		ID:     uuid.New(),
		Status: AckStatusReceived,
	}

	ack.Status = AckStatusProcessed
	assert.Equal(t, AckStatusProcessed, ack.Status)

	ack.Status = AckStatusRejected
	assert.Equal(t, AckStatusRejected, ack.Status)

	ack.Status = AckStatusFailed
	assert.Equal(t, AckStatusFailed, ack.Status)
}
