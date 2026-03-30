package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
)

func (s *MessageService) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	return s.repo.ListByTenant(ctx, tenantID, limit, offset)
}

func (s *MessageService) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	if s == nil || s.redis == nil {
		return ErrServiceUnavailable
	}
	if sub == nil {
		return ErrInvalidMessage
	}
	if sub.CreatedAt == 0 {
		sub.CreatedAt = time.Now().Unix()
	}
	return s.redis.CreateSubscription(ctx, sub)
}

func (s *MessageService) ListSubscriptions(ctx context.Context, agentID uuid.UUID) ([]model.Subscription, error) {
	if s == nil || s.redis == nil {
		return nil, ErrServiceUnavailable
	}
	return s.redis.ListSubscriptions(ctx, agentID)
}

func (s *MessageService) DeleteSubscription(ctx context.Context, agentID, subID uuid.UUID) error {
	if s == nil || s.redis == nil {
		return ErrServiceUnavailable
	}
	return s.redis.DeleteSubscription(ctx, agentID, subID)
}

func (s *AgentService) QueryByCapabilities(ctx context.Context, tenantID uuid.UUID, capabilities []string) ([]model.Agent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}

	agents, err := s.repo.QueryByCapabilities(ctx, tenantID, capabilities)
	if err != nil || len(capabilities) == 0 {
		return agents, err
	}

	required := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		required[strings.ToLower(capability)] = struct{}{}
	}

	filtered := make([]model.Agent, 0, len(agents))
	for _, agent := range agents {
		matched := false
		for _, capability := range agent.Capabilities {
			if _, ok := required[strings.ToLower(string(capability.Type))]; ok {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, agent)
		}
	}

	return filtered, nil
}

func (s *AgentService) ListAll(ctx context.Context) ([]model.Agent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListAll(ctx)
}

func (s *AgentService) GetByStatus(ctx context.Context, status model.AgentStatus) ([]model.Agent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
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
