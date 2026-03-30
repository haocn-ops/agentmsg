package engine

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"agentmsg/internal/model"
	"agentmsg/internal/observability"
	"agentmsg/internal/repository"
)

type MessageEngine struct {
	config  *EngineConfig
	db      *repository.PostgresDB
	redis   *repository.RedisClient
	router  *MessageRouter
	dlq     *DeadLetterQueue
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

var ErrEngineDependenciesNotConfigured = errors.New("engine dependencies not configured")
var ErrInvalidEngineMessage = errors.New("invalid message")

type EngineConfig struct {
	WorkerCount    int
	BatchSize      int
	FlushInterval  time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
}

func NewMessageEngine(cfg *EngineConfig, db *repository.PostgresDB, redis *repository.RedisClient) *MessageEngine {
	if cfg == nil {
		cfg = &EngineConfig{}
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 100 * time.Millisecond
	}

	return &MessageEngine{
		config: cfg,
		db:     db,
		redis:  redis,
		router: NewMessageRouter(),
		dlq:    NewDeadLetterQueue(redis, db, cfg.MaxRetries),
	}
}

func (e *MessageEngine) Start(ctx context.Context) error {
	if e.redis == nil {
		return ErrEngineDependenciesNotConfigured
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.mu.Unlock()

	e.wg.Add(e.config.WorkerCount + 1)
	for i := 0; i < e.config.WorkerCount; i++ {
		go e.processWorker()
	}
	go func() {
		defer e.wg.Done()
		e.dlq.ProcessLoop(e.ctx)
	}()

	return nil
}

func (e *MessageEngine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	cancel := e.cancel
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	e.wg.Wait()

	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
}

func (e *MessageEngine) processWorker() {
	defer e.wg.Done()

	idleDelay := e.config.FlushInterval
	if idleDelay <= 0 {
		idleDelay = 100 * time.Millisecond
	}

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			msg, err := e.redis.RPop(e.ctx, "message:pending")
			if err != nil {
				time.Sleep(idleDelay)
				continue
			}

			var message model.Message
			if err := json.Unmarshal([]byte(msg), &message); err != nil {
				slog.Error("Failed to unmarshal message", "error", err)
				continue
			}

			if err := e.routeMessage(e.ctx, &message); err != nil {
				slog.Error("Failed to route message", "message_id", message.ID, "error", err)
				if e.db != nil {
					_ = e.db.UpdateMessageStatus(e.ctx, message.ID, model.MessageStatusDeadLetter)
				}
				if dlqErr := e.dlq.Enqueue(e.ctx, &message, err.Error()); dlqErr != nil {
					slog.Error("Failed to enqueue dead letter", "message_id", message.ID, "error", dlqErr)
					if e.db != nil {
						_ = e.db.UpdateMessageStatus(e.ctx, message.ID, model.MessageStatusFailed)
					}
				}
			}
		}
	}
}

func (e *MessageEngine) routeMessage(ctx context.Context, msg *model.Message) error {
	start := time.Now()
	ctx, span := otel.Tracer("agentmsg/engine").Start(ctx, "message.route")
	defer span.End()
	routes, err := e.router.Route(msg)
	if err != nil {
		observability.RecordMessageOperation("route", "routing_error", time.Since(start))
		span.RecordError(err)
		return err
	}
	if len(routes) == 0 {
		observability.RecordMessageOperation("route", "no_routes", time.Since(start))
		span.SetAttributes(attribute.String("message.outcome", "no_routes"))
		return errors.New("no routes available for message")
	}

	var publishErr error
	for _, route := range routes {
		channel := "agent:" + route.RecipientID.String() + ":queue"
		if err := e.redis.Publish(ctx, channel, msg.ID.String()); err != nil {
			publishErr = err
			break
		}
	}

	if publishErr != nil {
		observability.RecordMessageOperation("route", "publish_error", time.Since(start))
		span.RecordError(publishErr)
		return publishErr
	}

	if e.db != nil {
		if err := e.db.UpdateMessageStatus(ctx, msg.ID, model.MessageStatusSent); err != nil {
			observability.RecordMessageOperation("route", "status_update_error", time.Since(start))
			span.RecordError(err)
			return err
		}
	}

	observability.RecordMessageOperation("route", "sent", time.Since(start))
	span.SetAttributes(
		attribute.String("message.id", msg.ID.String()),
		attribute.String("message.trace_id", msg.TraceID),
		attribute.Int("message.route_count", len(routes)),
		attribute.String("message.outcome", "sent"),
	)
	return nil
}

func (e *MessageEngine) SendMessage(ctx context.Context, msg *model.Message) (*model.SendResult, error) {
	if e.redis == nil {
		return nil, ErrEngineDependenciesNotConfigured
	}
	if msg == nil || len(msg.RecipientIDs) == 0 || len(msg.Content) == 0 {
		return nil, ErrInvalidEngineMessage
	}

	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.ConversationID == uuid.Nil {
		msg.ConversationID = uuid.New()
	}
	if msg.DeliveryGuarantee == "" {
		msg.DeliveryGuarantee = model.DeliveryAtLeastOnce
	}
	if msg.ContentType == "" {
		msg.ContentType = "application/json"
	}
	msg.ContentSize = len(msg.Content)

	if err := msg.SetRecipients(); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(msg)
	if err := e.redis.LPush(ctx, "message:pending", string(data)); err != nil {
		return nil, err
	}

	return &model.SendResult{
		MessageID: msg.ID,
		Status:    string(model.MessageStatusPending),
	}, nil
}

type MessageRouter struct {
	strategies map[string]RoutingStrategy
}

func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		strategies: make(map[string]RoutingStrategy),
	}
}

type RoutingStrategy interface {
	Route(msg *model.Message) ([]Route, error)
}

type Route struct {
	RecipientID uuid.UUID
	Channel     string
	Priority    int
}

func (r *MessageRouter) Route(msg *model.Message) ([]Route, error) {
	routes := make([]Route, 0, len(msg.RecipientIDs))
	for _, recipientID := range msg.RecipientIDs {
		routes = append(routes, Route{
			RecipientID: recipientID,
			Channel:     "agent:" + recipientID.String() + ":queue",
		})
	}
	return routes, nil
}

type DeadLetterQueue struct {
	redis      *repository.RedisClient
	db         *repository.PostgresDB
	maxRetries int
}

func NewDeadLetterQueue(redis *repository.RedisClient, db *repository.PostgresDB, maxRetries int) *DeadLetterQueue {
	return &DeadLetterQueue{
		redis:      redis,
		db:         db,
		maxRetries: maxRetries,
	}
}

func (q *DeadLetterQueue) ProcessLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.process(ctx)
		}
	}
}

func (q *DeadLetterQueue) process(ctx context.Context) {
	if q.db == nil || q.redis == nil {
		return
	}

	entries, err := q.db.ListRetryableDeadLetterEntries(ctx, 100)
	if err != nil {
		slog.Error("Failed to load dead letter entries", "error", err)
		return
	}

	for _, entry := range entries {
		q.retryEntry(ctx, entry)
	}
}

func (q *DeadLetterQueue) Enqueue(ctx context.Context, msg *model.Message, reason string) error {
	start := time.Now()
	data, _ := json.Marshal(map[string]interface{}{
		"message": msg,
		"reason":  reason,
		"time":    time.Now(),
	})

	if q.db != nil {
		entry := &model.DeadLetterEntry{
			ID:         uuid.New(),
			MessageID:  msg.ID,
			Reason:     reason,
			RetryCount: 0,
			MaxRetries: q.maxRetries,
			Payload:    data,
			Status:     model.DeadLetterStatusPending,
			CreatedAt:  time.Now(),
		}
		err := q.db.CreateDeadLetterEntry(ctx, entry)
		if err != nil {
			observability.RecordMessageOperation("dlq_enqueue", "db_error", time.Since(start))
			return err
		}
		observability.RecordMessageOperation("dlq_enqueue", "persisted", time.Since(start))
		return nil
	}

	if q.redis == nil {
		observability.RecordMessageOperation("dlq_enqueue", "service_unavailable", time.Since(start))
		return ErrEngineDependenciesNotConfigured
	}

	if err := q.redis.ZAdd(ctx, "dlq:pending", redis.Z{Score: float64(time.Now().Unix()), Member: string(data)}); err != nil {
		observability.RecordMessageOperation("dlq_enqueue", "redis_error", time.Since(start))
		return err
	}
	observability.RecordMessageOperation("dlq_enqueue", "queued", time.Since(start))
	return nil
}

func (q *DeadLetterQueue) retryEntry(ctx context.Context, entry model.DeadLetterEntry) {
	var envelope struct {
		Message *model.Message `json:"message"`
	}
	if err := json.Unmarshal(entry.Payload, &envelope); err != nil {
		slog.Error("Failed to decode dead letter payload", "entry_id", entry.ID, "error", err)
		q.markEntry(ctx, entry, model.DeadLetterStatusExhausted, entry.MaxRetries)
		return
	}
	if envelope.Message == nil {
		slog.Error("Dead letter payload missing message", "entry_id", entry.ID)
		q.markEntry(ctx, entry, model.DeadLetterStatusExhausted, entry.MaxRetries)
		return
	}

	if err := q.publishMessage(ctx, envelope.Message); err != nil {
		nextRetryCount := entry.RetryCount + 1
		status := model.DeadLetterStatusPending
		if nextRetryCount >= entry.MaxRetries {
			status = model.DeadLetterStatusExhausted
			if q.db != nil {
				_ = q.db.UpdateMessageStatus(ctx, envelope.Message.ID, model.MessageStatusFailed)
			}
		}
		q.markEntry(ctx, entry, status, nextRetryCount)
		slog.Error("Failed to retry dead letter message", "entry_id", entry.ID, "message_id", envelope.Message.ID, "error", err)
		observability.RecordMessageOperation("dlq_retry", "retry_failed", 0)
		return
	}

	if q.db != nil {
		_ = q.db.UpdateMessageStatus(ctx, envelope.Message.ID, model.MessageStatusSent)
	}
	q.markEntry(ctx, entry, model.DeadLetterStatusProcessed, entry.RetryCount+1)
	observability.RecordMessageOperation("dlq_retry", "retry_succeeded", 0)
}

func (q *DeadLetterQueue) publishMessage(ctx context.Context, msg *model.Message) error {
	if q.redis == nil {
		return ErrEngineDependenciesNotConfigured
	}
	if len(msg.RecipientIDs) == 0 && msg.RecipientStr != "" {
		if err := msg.ScanRecipients(); err != nil {
			return err
		}
	}
	if len(msg.RecipientIDs) == 0 {
		return errors.New("message has no recipients")
	}

	for _, recipientID := range msg.RecipientIDs {
		channel := "agent:" + recipientID.String() + ":queue"
		if err := q.redis.Publish(ctx, channel, msg.ID.String()); err != nil {
			return err
		}
	}

	return nil
}

func (q *DeadLetterQueue) markEntry(ctx context.Context, entry model.DeadLetterEntry, status model.DeadLetterStatus, retryCount int) {
	if q.db == nil {
		return
	}

	var processedAt *time.Time
	if status == model.DeadLetterStatusProcessed || status == model.DeadLetterStatusExhausted {
		now := time.Now()
		processedAt = &now
	}

	if err := q.db.UpdateDeadLetterEntry(ctx, entry.ID, status, retryCount, processedAt); err != nil {
		slog.Error("Failed to update dead letter entry", "entry_id", entry.ID, "error", err)
	}
}
