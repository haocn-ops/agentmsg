package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
)

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrServiceUnavailable = errors.New("service dependencies not configured")
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
	if s == nil || s.repo == nil || s.redis == nil {
		return ErrServiceUnavailable
	}
	if agent == nil {
		return ErrInvalidMessage
	}

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
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}

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
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	if agent == nil {
		return ErrInvalidMessage
	}

	agent.UpdatedAt = time.Now()
	return s.repo.Update(ctx, agent)
}

func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	return s.repo.Delete(ctx, id)
}

func (s *AgentService) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.Agent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListByTenant(ctx, tenantID)
}

func (s *AgentService) Heartbeat(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
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
	repo    *repository.MessageRepository
	ackRepo *repository.AcknowledgementRepository
	redis   *repository.RedisClient
}

func NewMessageService(repo *repository.MessageRepository, ackRepo *repository.AcknowledgementRepository, redis *repository.RedisClient) *MessageService {
	return &MessageService{
		repo:    repo,
		ackRepo: ackRepo,
		redis:   redis,
	}
}

func (s *MessageService) Send(ctx context.Context, msg *model.Message) (*model.SendResult, error) {
	if s == nil || s.repo == nil || s.redis == nil {
		return nil, ErrServiceUnavailable
	}
	if msg == nil || len(msg.RecipientIDs) == 0 || len(msg.Content) == 0 {
		return nil, ErrInvalidMessage
	}

	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.ConversationID == uuid.Nil {
		msg.ConversationID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.Status == "" {
		msg.Status = model.MessageStatusPending
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

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	if err := s.redis.LPush(ctx, "message:pending", string(payload)); err != nil {
		return nil, err
	}

	return &model.SendResult{
		MessageID: msg.ID,
		Status:    string(model.MessageStatusPending),
	}, nil
}

func (s *MessageService) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.GetByID(ctx, id)
}

func (s *MessageService) Acknowledge(ctx context.Context, ack *model.Acknowledgement) error {
	if s == nil || s.repo == nil || s.ackRepo == nil {
		return ErrServiceUnavailable
	}
	if ack == nil {
		return ErrInvalidMessage
	}
	if ack.MessageID == uuid.Nil || ack.AgentID == uuid.Nil || ack.Status == "" {
		return ErrInvalidMessage
	}

	msg, err := s.repo.GetByID(ctx, ack.MessageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return ErrInvalidMessage
	}

	if ack.ID == uuid.Nil {
		ack.ID = uuid.New()
	}
	if ack.CreatedAt.IsZero() {
		ack.CreatedAt = time.Now()
	}
	if ack.Nonce == "" {
		ack.Nonce = uuid.NewString()
	}

	if err := s.ackRepo.Create(ctx, ack); err != nil {
		return err
	}

	return s.repo.UpdateStatus(ctx, ack.MessageID, mapAckToMessageStatus(ack.Status))
}

func (s *MessageService) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListByConversation(ctx, conversationID, limit)
}

type AuthService struct {
	jwtSecret []byte
}

func NewAuthService(jwtSecret string) *AuthService {
	return &AuthService{jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) GenerateToken(agentID uuid.UUID, tenantID uuid.UUID) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", ErrServiceUnavailable
	}

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	payload := strings.Join([]string{
		agentID.String(),
		tenantID.String(),
		strconv.FormatInt(expiresAt, 10),
	}, ".")

	mac := hmac.New(sha256.New, s.jwtSecret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return encodedPayload + "." + signature, nil
}

func (s *AuthService) ValidateToken(token string) (*model.TokenClaims, error) {
	if len(s.jwtSecret) == 0 {
		return nil, ErrServiceUnavailable
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expectedMAC := hmac.New(sha256.New, s.jwtSecret)
	if _, err := expectedMAC.Write(payloadBytes); err != nil {
		return nil, err
	}

	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal(signatureBytes, expectedMAC.Sum(nil)) {
		return nil, ErrInvalidToken
	}

	fields := strings.Split(string(payloadBytes), ".")
	if len(fields) != 3 {
		return nil, ErrInvalidToken
	}

	agentID, err := uuid.Parse(fields[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	tenantID, err := uuid.Parse(fields[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expiresAt, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return nil, ErrInvalidToken
	}

	return &model.TokenClaims{
		AgentID:  agentID,
		TenantID: tenantID,
	}, nil
}

func mapAckToMessageStatus(status model.AckStatus) model.MessageStatus {
	switch status {
	case model.AckStatusReceived:
		return model.MessageStatusDelivered
	case model.AckStatusProcessed:
		return model.MessageStatusProcessed
	case model.AckStatusRejected, model.AckStatusFailed:
		return model.MessageStatusFailed
	default:
		return model.MessageStatusPending
	}
}
