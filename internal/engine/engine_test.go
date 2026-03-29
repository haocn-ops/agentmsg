package engine

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"agentmsg/internal/model"
)

func TestMessageEngineCreation(t *testing.T) {
	cfg := &EngineConfig{
		WorkerCount:     16,
		BatchSize:      100,
		FlushInterval:  100 * time.Millisecond,
		MaxRetries:     3,
		RetryBaseDelay: time.Second,
	}

	engine := NewMessageEngine(cfg, nil, nil)

	assert.NotNil(t, engine)
	assert.Equal(t, cfg.WorkerCount, engine.config.WorkerCount)
	assert.Equal(t, cfg.BatchSize, engine.config.BatchSize)
}

func TestMessageRouterDirectRoute(t *testing.T) {
	router := NewMessageRouter()

	recipientID := uuid.New()
	msg := &model.Message{
		ID:           uuid.New(),
		RecipientIDs: []uuid.UUID{recipientID},
	}

	routes, err := router.Route(msg)
	assert.NoError(t, err)
	assert.Len(t, routes, 1)
	assert.Equal(t, recipientID, routes[0].RecipientID)
	assert.Contains(t, routes[0].Channel, recipientID.String())
}

func TestMessageRouterMultipleRecipients(t *testing.T) {
	router := NewMessageRouter()

	recipient1 := uuid.New()
	recipient2 := uuid.New()
	recipient3 := uuid.New()

	msg := &model.Message{
		ID:           uuid.New(),
		RecipientIDs: []uuid.UUID{recipient1, recipient2, recipient3},
	}

	routes, err := router.Route(msg)
	assert.NoError(t, err)
	assert.Len(t, routes, 3)
}

func TestDeadLetterQueueCreation(t *testing.T) {
	dlq := NewDeadLetterQueue(nil, nil, 3)
	assert.NotNil(t, dlq)
	assert.Equal(t, 3, dlq.maxRetries)
}

func TestMessageEngineConfig(t *testing.T) {
	cfg := &EngineConfig{
		WorkerCount:     32,
		BatchSize:      200,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     5,
		RetryBaseDelay: 500 * time.Millisecond,
	}

	engine := NewMessageEngine(cfg, nil, nil)

	assert.Equal(t, 32, engine.config.WorkerCount)
	assert.Equal(t, 200, engine.config.BatchSize)
	assert.Equal(t, 5, engine.config.MaxRetries)
}

func TestSendResultCreation(t *testing.T) {
	msgID := uuid.New()
	now := time.Now().UnixMilli()

	result := &model.SendResult{
		MessageID:   msgID,
		Status:      string(model.MessageStatusSent),
		DeliveredAt: &now,
	}

	assert.Equal(t, msgID, result.MessageID)
	assert.Equal(t, "sent", result.Status)
	assert.NotNil(t, result.DeliveredAt)
}
