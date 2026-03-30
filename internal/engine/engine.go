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

	"agentmsg/internal/model"
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
	WorkerCount     int
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
		config:  cfg,
		db:      db,
		redis:   redis,
		router:  NewMessageRouter(),
		dlq:     NewDeadLetterQueue(redis, db, cfg.MaxRetries),
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

			e.routeMessage(&message)
		}
	}
}

func (e *MessageEngine) routeMessage(msg *model.Message) {
	routes, err := e.router.Route(msg)
	if err != nil {
		slog.Error("Failed to route message", "error", err)
		return
	}

	for _, route := range routes {
		channel := "agent:" + route.RecipientID.String() + ":queue"
		if err := e.redis.Publish(e.ctx, channel, msg.ID.String()); err != nil {
			slog.Error("Failed to publish to channel", "channel", channel, "error", err)
		}
	}
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
	Channel    string
	Priority   int
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
			q.process()
		}
	}
}

func (q *DeadLetterQueue) process() {
}

func (q *DeadLetterQueue) Enqueue(ctx context.Context, msg *model.Message, reason string) error {
	data, _ := json.Marshal(map[string]interface{}{
		"message": msg,
		"reason":  reason,
		"time":    time.Now(),
	})
	return q.redis.ZAdd(ctx, "dlq:pending", redis.Z{Score: float64(time.Now().Unix()), Member: string(data)})
}
