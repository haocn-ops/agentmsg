package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrUnauthorized  = errors.New("unauthorized")
)

type AgentService struct {
	repo   *repository.AgentRepository
	redis  *repository.RedisClient
}

func NewAgentService(repo *repository.AgentRepository, redis *repository.RedisClient) *AgentService {
	return &AgentService{
		repo:  repo,
		redis: redis,
	}
}

func (s *AgentService) Register(ctx context.Context, agent *model.Agent) error {
	agent.ID = uuid.New()
	agent.Status = model.AgentStatusOnline
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	if err := s.repo.Create(ctx, agent); err != nil {
		return err
	}

	return s.redis.Publish(ctx, "agents:created", agent.ID.String())
}

func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*model.Agent, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (s *AgentService) Update(ctx context.Context, agent *model.Agent) error {
	agent.UpdatedAt = time.Now()
	return s.repo.Update(ctx, agent)
}

func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *AgentService) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.Agent, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

func (s *AgentService) Heartbeat(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateStatus(ctx, id, model.AgentStatusOnline)
}

func (s *AgentService) UpdateCapabilities(ctx context.Context, id uuid.UUID, capabilities model.Capabilities) error {
	agent, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	agent.Capabilities = capabilities
	return s.Update(ctx, agent)
}

type MessageService struct {
	repo  *repository.MessageRepository
	redis *repository.RedisClient
}

func NewMessageService(repo *repository.MessageRepository, redis *repository.RedisClient) *MessageService {
	return &MessageService{
		repo:  repo,
		redis: redis,
	}
}

func (s *MessageService) Send(ctx context.Context, msg *model.Message) (*model.SendResult, error) {
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()
	msg.Status = model.MessageStatusPending

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}

	for _, recipientID := range msg.RecipientIDs {
		channel := "agent:" + recipientID.String() + ":queue"
		if err := s.redis.Publish(ctx, channel, msg.ID.String()); err != nil {
			continue
		}
	}

	return &model.SendResult{
		MessageID: msg.ID,
		Status:    string(model.MessageStatusSent),
	}, nil
}

func (s *MessageService) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MessageService) Acknowledge(ctx context.Context, id uuid.UUID, status model.MessageStatus) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *MessageService) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	return s.repo.ListByConversation(ctx, conversationID, limit)
}

type AuthService struct {
	jwtSecret []byte
}

func NewAuthService(jwtSecret string) *AuthService {
	return &AuthService{jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) GenerateToken(agentID uuid.UUID, tenantID uuid.UUID) (string, error) {
	return "token-" + agentID.String(), nil
}

func (s *AuthService) ValidateToken(token string) (*model.TokenClaims, error) {
	return &model.TokenClaims{
		AgentID:  uuid.New(),
		TenantID: uuid.New(),
	}, nil
}
