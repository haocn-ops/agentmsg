package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/engine"
	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

func setupTestEngine(t *testing.T) (*engine.MessageEngine, *repository.PostgresDB, func()) {
	cfg := &engine.EngineConfig{
		WorkerCount:     2,
		BatchSize:      10,
		FlushInterval:  100 * time.Millisecond,
		MaxRetries:     3,
		RetryBaseDelay: 10 * time.Millisecond,
	}

	redis := &repository.RedisClient{}
	db := &repository.PostgresDB{}

	eng := engine.NewMessageEngine(cfg, db, redis)

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	cleanup := func() {
		eng.Stop()
	}

	return eng, db, cleanup
}

func TestMessageEngineSendMessage(t *testing.T) {
	eng, _, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   model.MessageTypeGeneric,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte("test content"),
		ContentSize:   12,
		ContentType:   "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:      uuid.New(),
		CreatedAt:    time.Now(),
	}

	result, err := eng.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if result.MessageID != msg.ID {
		t.Errorf("Expected message ID %s, got %s", msg.ID, result.MessageID)
	}

	if result.Status != string(model.MessageStatusPending) {
		t.Errorf("Expected status 'pending', got '%s'", result.Status)
	}
}

func TestExactlyOnceEngine(t *testing.T) {
	cfg := &engine.ExactlyOnceConfig{
		DeduplicationWindow: 1 * time.Hour,
		MaxCacheSize:       1000,
		CleanupInterval:     1 * time.Minute,
	}

	redis := &repository.RedisClient{}
	db := &repository.PostgresDB{}

	eng := engine.NewExactlyOnceEngine(cfg, db, redis)

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}
	defer eng.Stop()

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   model.MessageTypeGeneric,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte("test content"),
		ContentSize:   12,
		ContentType:   "text/plain",
		DeliveryGuarantee: model.DeliveryExactlyOnce,
		TenantID:      uuid.New(),
		CreatedAt:    time.Now(),
	}

	duplicate, err := eng.IsDuplicate(ctx, msg)
	if err != nil {
		t.Fatalf("IsDuplicate failed: %v", err)
	}
	if duplicate {
		t.Error("Expected first message to not be duplicate")
	}

	duplicate, err = eng.IsDuplicate(ctx, msg)
	if err != nil {
		t.Fatalf("IsDuplicate failed for duplicate check: %v", err)
	}
	if !duplicate {
		t.Error("Expected second message to be duplicate")
	}
}

func TestIdempotencyTracker(t *testing.T) {
	redis := &repository.RedisClient{}
	tracker := engine.NewIdempotencyTracker(redis)

	ctx := context.Background()

	key := "test-idempotency-key"
	resultID := uuid.New().String()

	exists, err := tracker.Check(ctx, key)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist initially")
	}

	err = tracker.Record(ctx, key, resultID, 60)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	exists, err = tracker.Check(ctx, key)
	if err != nil {
		t.Fatalf("Check failed after record: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist after record")
	}

	storedResult, err := tracker.GetResult(ctx, key)
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if storedResult == "" {
		t.Error("Expected non-empty result")
	}
}

func TestMessageRouting(t *testing.T) {
	eng, _, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	recipient1 := uuid.New()
	recipient2 := uuid.New()

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   model.MessageTypeTaskRequest,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{recipient1, recipient2},
		Content:       []byte("routing test"),
		ContentSize:   13,
		ContentType:   "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:      uuid.New(),
		CreatedAt:    time.Now(),
	}

	_, err := eng.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestDeadLetterQueue(t *testing.T) {
	cfg := &engine.EngineConfig{
		WorkerCount:     1,
		BatchSize:      10,
		FlushInterval:  100 * time.Millisecond,
		MaxRetries:     2,
		RetryBaseDelay: 10 * time.Millisecond,
	}

	redis := &repository.RedisClient{}
	db := &repository.PostgresDB{}

	eng := engine.NewMessageEngine(cfg, db, redis)

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}
	defer eng.Stop()

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		MessageType:   model.MessageTypeGeneric,
		SenderID:      uuid.New(),
		RecipientIDs:  []uuid.UUID{uuid.New()},
		Content:       []byte("dlq test"),
		ContentSize:   8,
		ContentType:   "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:      uuid.New(),
		CreatedAt:    time.Now(),
	}

	_, err := eng.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}