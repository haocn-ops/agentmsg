package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

type ExactlyOnceEngine struct {
	config       *ExactlyOnceConfig
	db           *repository.PostgresDB
	redis        *repository.RedisClient
	processedIDs map[string]time.Time
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

type ExactlyOnceConfig struct {
	DeduplicationWindow time.Duration
	MaxCacheSize        int
	CleanupInterval     time.Duration
}

func NewExactlyOnceEngine(cfg *ExactlyOnceConfig, db *repository.PostgresDB, redis *repository.RedisClient) *ExactlyOnceEngine {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &ExactlyOnceEngine{
		config:       cfg,
		db:           db,
		redis:        redis,
		processedIDs: make(map[string]time.Time),
		ctx:          ctx,
		cancel:       cancel,
	}

	if cfg.CleanupInterval > 0 {
		go engine.cleanupLoop()
	}

	return engine
}

func (e *ExactlyOnceEngine) Start(ctx context.Context) error {
	return nil
}

func (e *ExactlyOnceEngine) Stop() {
	e.cancel()
}

func (e *ExactlyOnceEngine) IsDuplicate(ctx context.Context, msg *model.Message) (bool, error) {
	if e.redis == nil {
		return false, ErrEngineDependenciesNotConfigured
	}

	key := e.generateDeduplicationKey(msg)

	exists, err := e.redis.Exists(ctx, "dedup:"+key)
	if err != nil {
		exists, err := e.checkInDB(ctx, msg.ID)
		if err != nil {
			return false, err
		}
		return exists, nil
	}

	if exists {
		return true, nil
	}

	if err := e.markAsProcessed(ctx, msg); err != nil {
		return false, err
	}

	return false, nil
}

func (e *ExactlyOnceEngine) markAsProcessed(ctx context.Context, msg *model.Message) error {
	key := e.generateDeduplicationKey(msg)

	ttl := e.config.DeduplicationWindow
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	msgData, _ := json.Marshal(msg)
	return e.redis.SetWithExpiry(ctx, "dedup:"+key, string(msgData), int(ttl.Seconds()))
}

func (e *ExactlyOnceEngine) generateDeduplicationKey(msg *model.Message) string {
	data := msg.SenderID.String() + ":" + msg.ConversationID.String()
	if msg.TaskContext != nil {
		data += ":" + msg.TaskContext.TaskID.String()
	}
	data += ":" + string(msg.Content)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

func (e *ExactlyOnceEngine) checkInDB(ctx context.Context, messageID uuid.UUID) (bool, error) {
	if e.db == nil {
		return false, ErrEngineDependenciesNotConfigured
	}

	msg, err := e.db.GetMessageByID(ctx, messageID)
	if err != nil {
		return false, err
	}
	return msg != nil, nil
}

func (e *ExactlyOnceEngine) cleanupLoop() {
	ticker := time.NewTicker(e.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.cleanup()
		}
	}
}

func (e *ExactlyOnceEngine) cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for key, timestamp := range e.processedIDs {
		if now.Sub(timestamp) > e.config.DeduplicationWindow {
			delete(e.processedIDs, key)
		}
	}
}

func (e *ExactlyOnceEngine) GetDeliveryStatus(ctx context.Context, messageID uuid.UUID) (model.MessageStatus, error) {
	if e.db == nil {
		return "", ErrEngineDependenciesNotConfigured
	}

	msg, err := e.db.GetMessageByID(ctx, messageID)
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", nil
	}
	return msg.Status, nil
}

func (e *ExactlyOnceEngine) RecordAcknowledgement(ctx context.Context, ack *model.Acknowledgement) error {
	if e.db == nil {
		return ErrEngineDependenciesNotConfigured
	}
	return e.db.CreateAcknowledgement(ctx, ack)
}

func (e *ExactlyOnceEngine) VerifyAcknowledgement(ctx context.Context, messageID uuid.UUID, nonce string) (bool, error) {
	if e.db == nil {
		return false, ErrEngineDependenciesNotConfigured
	}

	ack, err := e.db.GetAcknowledgement(ctx, messageID)
	if err != nil {
		return false, err
	}
	if ack == nil {
		return false, nil
	}
	return ack.Nonce == nonce, nil
}

type IdempotencyTracker struct {
	redis *repository.RedisClient
}

func NewIdempotencyTracker(redis *repository.RedisClient) *IdempotencyTracker {
	return &IdempotencyTracker{redis: redis}
}

func (t *IdempotencyTracker) Check(ctx context.Context, idempotencyKey string) (bool, error) {
	if t.redis == nil {
		return false, ErrEngineDependenciesNotConfigured
	}
	key := "idempotency:" + idempotencyKey
	exists, err := t.redis.Exists(ctx, key)
	return exists, err
}

func (t *IdempotencyTracker) Record(ctx context.Context, idempotencyKey string, resultID string, ttlSeconds int) error {
	if t.redis == nil {
		return ErrEngineDependenciesNotConfigured
	}
	key := "idempotency:" + idempotencyKey
	value := resultID + ":" + time.Now().Format(time.RFC3339Nano)
	return t.redis.SetWithExpiry(ctx, key, value, ttlSeconds)
}

func (t *IdempotencyTracker) GetResult(ctx context.Context, idempotencyKey string) (string, error) {
	if t.redis == nil {
		return "", ErrEngineDependenciesNotConfigured
	}
	return t.redis.Get(ctx, "idempotency:"+idempotencyKey)
}
