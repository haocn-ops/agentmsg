package engine

import (
	"context"
	"encoding/json"
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

type EngineConfig struct {
	WorkerCount     int
	BatchSize      int
	FlushInterval  time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
}

func NewMessageEngine(cfg *EngineConfig, db *repository.PostgresDB, redis *repository.RedisClient) *MessageEngine {
	ctx, cancel := context.WithCancel(context.Background())

	return &MessageEngine{
		config:  cfg,
		db:      db,
		redis:   redis,
		router:  NewMessageRouter(),
		dlq:     NewDeadLetterQueue(redis, db, cfg.MaxRetries),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (e *MessageEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = true
	e.mu.Unlock()

	e.wg.Add(2)
	go e.messageProcessor()
	go e.dlq.ProcessLoop()

	return nil
}

func (e *MessageEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.cancel()
	e.wg.Wait()
	e.running = false
}

func (e *MessageEngine) messageProcessor() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.processBatch()
		}
	}
}

func (e *MessageEngine) processBatch() {
	for i := 0; i < e.config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.processWorker()
	}
}

func (e *MessageEngine) processWorker() {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			msg, err := e.redis.RPop(e.ctx, "message:pending")
			if err != nil {
				time.Sleep(100 * time.Millisecond)
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
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()

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

func (q *DeadLetterQueue) ProcessLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
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
