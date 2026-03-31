package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

type BillingService struct {
	repo     *repository.BillingRepository
	redis    *repository.RedisClient
	planRepo *repository.PlanRepository
}

func NewBillingService(repo *repository.BillingRepository, redis *repository.RedisClient, planRepo *repository.PlanRepository) *BillingService {
	return &BillingService{
		repo:     repo,
		redis:    redis,
		planRepo: planRepo,
	}
}

func (s *BillingService) RecordMessageSent(ctx context.Context, tenantID, agentID uuid.UUID, messageID uuid.UUID) error {
	event := &model.BillingEvent{
		ID:          uuid.New(),
		TenantID:    tenantID,
		AgentID:     agentID,
		EventType:   model.BillingEventMessageSent,
		MessageID:   &messageID,
		Quantity:    1,
		UnitPrice:   0.001,
		TotalAmount: 0.001,
		Currency:    "USD",
		PeriodStart: getBillingPeriodStart(),
		PeriodEnd:   getBillingPeriodEnd(),
		Status:      model.BillingStatusPending,
		CreatedAt:   time.Now(),
	}

	return s.repo.CreateEvent(ctx, event)
}

func (s *BillingService) GetUsageStats(ctx context.Context, tenantID uuid.UUID) (*model.TenantUsage, error) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	messages, err := s.repo.CountMessagesInPeriod(ctx, tenantID, periodStart)
	if err != nil {
		return nil, err
	}

	return &model.TenantUsage{
		MessageCount: messages,
		ApiCalls:     messages,
		ResetAt:      getBillingPeriodEnd().Unix(),
	}, nil
}

func (s *BillingService) GenerateInvoice(ctx context.Context, tenantID uuid.UUID, periodStart, periodEnd time.Time) (*model.Invoice, error) {
	events, err := s.repo.GetEventsInPeriod(ctx, tenantID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	lineItems := make([]model.InvoiceLineItem, 0)
	subtotal := 0.0

	messageCount := 0
	for _, event := range events {
		if event.EventType == model.BillingEventMessageSent {
			messageCount += event.Quantity
		}
	}

	if messageCount > 0 {
		lineItems = append(lineItems, model.InvoiceLineItem{
			Description: "Message Delivery",
			Quantity:    messageCount,
			UnitPrice:   0.001,
			Amount:      float64(messageCount) * 0.001,
		})
		subtotal += float64(messageCount) * 0.001
	}

	tax := subtotal * 0.1
	total := subtotal + tax

	invoice := &model.Invoice{
		ID:            uuid.New(),
		TenantID:      tenantID,
		InvoiceNumber: generateInvoiceNumber(),
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		Subtotal:      subtotal,
		Tax:           tax,
		Total:         total,
		Currency:      "USD",
		Status:        model.InvoiceStatusPending,
		DueDate:       periodEnd.AddDate(0, 0, 30),
		LineItems:     lineItems,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.CreateInvoice(ctx, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

func (s *BillingService) GetPricingPlans(ctx context.Context) ([]model.PricingPlan, error) {
	return s.planRepo.ListActivePlans(ctx)
}

func (s *BillingService) CheckQuota(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}

	usage, err := s.GetUsageStats(ctx, tenantID)
	if err != nil {
		return false, err
	}

	return usage.MessageCount < tenant.Limits.MessagePerMonth, nil
}

func getBillingPeriodStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func getBillingPeriodEnd() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
}

func generateInvoiceNumber() string {
	now := time.Now()
	return "INV-" + now.Format("200601") + "-" + uuid.New().String()[:8]
}

type IdempotencyService struct {
	redis *repository.RedisClient
}

func NewIdempotencyService(redis *repository.RedisClient) *IdempotencyService {
	return &IdempotencyService{redis: redis}
}

func (s *IdempotencyService) CheckAndSet(ctx context.Context, key string, messageID uuid.UUID, ttl time.Duration) (bool, error) {
	combinedKey := "idempotency:" + key

	existing, err := s.redis.Get(ctx, combinedKey)
	if err == nil && existing != "" {
		return false, nil
	}

	value := messageID.String() + ":" + time.Now().Format(time.RFC3339)
	if err := s.redis.Set(ctx, combinedKey, value); err != nil {
		return false, err
	}

	if ttl > 0 {
		if err := s.redis.Expire(ctx, combinedKey, int(ttl.Seconds())); err != nil {
			_ = s.redis.Del(ctx, combinedKey)
			return false, err
		}
	}

	return true, nil
}

func GenerateIdempotencyKey(senderID, recipientID uuid.UUID, content []byte) string {
	hash := sha256.Sum256(append(append(senderID[:], recipientID[:]...), content...))
	return hex.EncodeToString(hash[:16])
}
