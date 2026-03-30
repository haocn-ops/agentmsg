package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

func setupTestEngine(t *testing.T) (*MessageEngine, func()) {
	t.Helper()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}

	cfg := &EngineConfig{
		WorkerCount:     2,
		BatchSize:       10,
		FlushInterval:   100 * time.Millisecond,
		MaxRetries:      3,
		RetryBaseDelay:  10 * time.Millisecond,
	}

	redis, err := repository.NewRedisClient(redisURL)
	require.NoError(t, err)

	eng := NewMessageEngine(cfg, nil, redis)

	ctx := context.Background()
	require.NoError(t, eng.Start(ctx))

	cleanup := func() {
		eng.Stop()
		require.NoError(t, redis.Close())
	}

	return eng, cleanup
}

func TestMessageEngineSendMessage(t *testing.T) {
	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	msg := &model.Message{
		ID:               uuid.New(),
		ConversationID:   uuid.New(),
		MessageType:      model.MessageTypeGeneric,
		SenderID:         uuid.New(),
		RecipientIDs:     []uuid.UUID{uuid.New()},
		Content:          []byte("test content"),
		ContentSize:      12,
		ContentType:      "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:         uuid.New(),
		CreatedAt:        time.Now(),
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
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}

	cfg := &ExactlyOnceConfig{
		DeduplicationWindow: 1 * time.Hour,
		MaxCacheSize:       1000,
		CleanupInterval:     1 * time.Minute,
	}

	redis, err := repository.NewRedisClient(redisURL)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, redis.Close())
	}()

	eng := NewExactlyOnceEngine(cfg, nil, redis)

	ctx := context.Background()
	require.NoError(t, eng.Start(ctx))
	defer eng.Stop()

	msg := &model.Message{
		ID:               uuid.New(),
		ConversationID:   uuid.New(),
		MessageType:      model.MessageTypeGeneric,
		SenderID:         uuid.New(),
		RecipientIDs:     []uuid.UUID{uuid.New()},
		Content:          []byte("test content"),
		ContentSize:      12,
		ContentType:      "text/plain",
		DeliveryGuarantee: model.DeliveryExactlyOnce,
		TenantID:         uuid.New(),
		CreatedAt:        time.Now(),
	}

	duplicate, err := eng.IsDuplicate(ctx, msg)
	require.NoError(t, err)
	if duplicate {
		t.Error("Expected first message to not be duplicate")
	}

	duplicate, err = eng.IsDuplicate(ctx, msg)
	require.NoError(t, err)
	if !duplicate {
		t.Error("Expected second message to be duplicate")
	}
}

func TestIdempotencyTracker(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}

	redis, err := repository.NewRedisClient(redisURL)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, redis.Close())
	}()

	tracker := NewIdempotencyTracker(redis)

	ctx := context.Background()

	key := "test-idempotency-key"
	resultID := uuid.New().String()

	exists, err := tracker.Check(ctx, key)
	require.NoError(t, err)
	if exists {
		t.Error("Expected key to not exist initially")
	}

	err = tracker.Record(ctx, key, resultID, 60)
	require.NoError(t, err)

	exists, err = tracker.Check(ctx, key)
	require.NoError(t, err)
	if !exists {
		t.Error("Expected key to exist after record")
	}

	storedResult, err := tracker.GetResult(ctx, key)
	require.NoError(t, err)
	if storedResult == "" {
		t.Error("Expected non-empty result")
	}
}

func TestMessageRouting(t *testing.T) {
	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	recipient1 := uuid.New()
	recipient2 := uuid.New()

	msg := &model.Message{
		ID:               uuid.New(),
		ConversationID:   uuid.New(),
		MessageType:      model.MessageTypeTaskRequest,
		SenderID:         uuid.New(),
		RecipientIDs:     []uuid.UUID{recipient1, recipient2},
		Content:          []byte("routing test"),
		ContentSize:      13,
		ContentType:      "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:         uuid.New(),
		CreatedAt:        time.Now(),
	}

	_, err := eng.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestDeadLetterQueue(t *testing.T) {
	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	ctx := context.Background()

	msg := &model.Message{
		ID:               uuid.New(),
		ConversationID:   uuid.New(),
		MessageType:      model.MessageTypeGeneric,
		SenderID:         uuid.New(),
		RecipientIDs:     []uuid.UUID{uuid.New()},
		Content:          []byte("dlq test"),
		ContentSize:      8,
		ContentType:      "text/plain",
		DeliveryGuarantee: model.DeliveryAtLeastOnce,
		TenantID:         uuid.New(),
		CreatedAt:        time.Now(),
	}

	_, err := eng.SendMessage(ctx, msg)
	require.NoError(t, err)
}
