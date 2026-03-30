package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub      string `json:"sub"`
	AgentID  string `json:"agent_id"`
	TenantID string `json:"tenant_id"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

func NewAuthService(jwtSecret string) *AuthService {
	return &AuthService{jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) GenerateToken(agentID uuid.UUID, tenantID uuid.UUID) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", ErrServiceUnavailable
	}

	now := time.Now().Unix()
	headerBytes, err := json.Marshal(jwtHeader{
		Alg: "HS256",
		Typ: "JWT",
	})
	if err != nil {
		return "", err
	}

	claimsBytes, err := json.Marshal(jwtClaims{
		Sub:       agentID.String(),
		AgentID:   agentID.String(),
		TenantID:  tenantID.String(),
		IssuedAt:  now,
		ExpiresAt: now + int64((24 * time.Hour).Seconds()),
	})
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := encodedHeader + "." + encodedClaims

	signature, err := s.signJWT(signingInput)
	if err != nil {
		return "", err
	}

	return signingInput + "." + signature, nil
}

func (s *AuthService) ValidateToken(token string) (*model.TokenClaims, error) {
	if len(s.jwtSecret) == 0 {
		return nil, ErrServiceUnavailable
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}
	if header.Alg != "HS256" || header.Typ != "JWT" {
		return nil, ErrInvalidToken
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expectedSignature, err := s.signJWT(parts[0] + "." + parts[1])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return nil, ErrInvalidToken
	}

	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	agentID, err := uuid.Parse(claims.AgentID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims.ExpiresAt == 0 || time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrInvalidToken
	}

	return &model.TokenClaims{
		AgentID:   agentID,
		TenantID:  tenantID,
		IssuedAt:  claims.IssuedAt,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

func (s *AuthService) signJWT(signingInput string) (string, error) {
	mac := hmac.New(sha256.New, s.jwtSecret)
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
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
