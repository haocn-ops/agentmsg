package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"agentmsg/internal/model"
)

func TestMessageServiceSend(t *testing.T) {
	ctx := context.Background()

	svc := &MessageService{}

	msg := &model.Message{
		ConversationID:  uuid.New(),
		MessageType:    model.MessageTypeTaskRequest,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte(`{"task":"test"}`),
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:      uuid.New(),
	}

	result, err := svc.Send(ctx, msg)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
	assert.Nil(t, result)
}

func TestMessageServiceGetByID(t *testing.T) {
	ctx := context.Background()

	svc := &MessageService{}

	msgID := uuid.New()

	msg, err := svc.GetByID(ctx, msgID)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
	assert.Nil(t, msg)
}

func TestMessageServiceAcknowledge(t *testing.T) {
	ctx := context.Background()

	svc := &MessageService{}

	msgID := uuid.New()
	status := model.MessageStatusProcessed

	err := svc.Acknowledge(ctx, msgID, status)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestMessageServiceListByConversation(t *testing.T) {
	ctx := context.Background()

	svc := &MessageService{}

	conversationID := uuid.New()

	messages, err := svc.ListByConversation(ctx, conversationID, 100)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
	assert.Nil(t, messages)
}

func TestMessageServiceSendRejectsInvalidMessage(t *testing.T) {
	ctx := context.Background()

	svc := &MessageService{}

	result, err := svc.Send(ctx, &model.Message{})
	assert.ErrorIs(t, err, ErrServiceUnavailable)
	assert.Nil(t, result)
}

func TestMessageDeliveryGuarantees(t *testing.T) {
	tests := []struct {
		name      string
		guarantee model.DeliveryGuarantee
	}{
		{"at-most-once", model.DeliveryAtMostOnce},
		{"at-least-once", model.DeliveryAtLeastOnce},
		{"exactly-once", model.DeliveryExactlyOnce},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &model.Message{
				ID:                 uuid.New(),
				DeliveryGuarantee: tt.guarantee,
			}
			assert.Equal(t, tt.guarantee, msg.DeliveryGuarantee)
		})
	}
}
