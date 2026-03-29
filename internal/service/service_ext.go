package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

func (s *MessageService) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.repo.ListByTenant(ctx, tenantID, limit, offset)
}

func (s *MessageService) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	return s.redis.CreateSubscription(ctx, sub)
}

func (s *MessageService) ListSubscriptions(ctx context.Context, agentID uuid.UUID) ([]model.Subscription, error) {
	return s.redis.ListSubscriptions(ctx, agentID)
}

func (s *MessageService) DeleteSubscription(ctx context.Context, agentID, subID uuid.UUID) error {
	return s.redis.DeleteSubscription(ctx, agentID, subID)
}

func (s *AgentService) QueryByCapabilities(ctx context.Context, tenantID uuid.UUID, capabilities []string) ([]model.Agent, error) {
	return s.repo.QueryByCapabilities(ctx, tenantID, capabilities)
}

func (s *AgentService) ListAll(ctx context.Context) ([]model.Agent, error) {
	return s.repo.ListAll(ctx)
}

func (s *AgentService) GetByStatus(ctx context.Context, status model.AgentStatus) ([]model.Agent, error) {
	return s.repo.GetByStatus(ctx, status)
}

func (s *AgentService) GetStats(ctx context.Context, tenantID uuid.UUID) (*model.AgentStats, error) {
	agents, err := s.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	stats := &model.AgentStats{
		Total:     len(agents),
		Timestamp: time.Now().Unix(),
	}

	for _, agent := range agents {
		switch agent.Status {
		case model.AgentStatusOnline:
			stats.Online++
		case model.AgentStatusAway:
			stats.Away++
		case model.AgentStatusBusy:
			stats.Busy++
		case model.AgentStatusOffline:
			stats.Offline++
		}
	}

	return stats, nil
}