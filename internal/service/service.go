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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"agentmsg/internal/model"
	"agentmsg/internal/observability"
	"agentmsg/internal/repository"
)

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentAlreadyExists = errors.New("agent already exists")
	ErrMessageNotFound    = errors.New("message not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrServiceUnavailable = errors.New("service dependencies not configured")
)

type AgentService struct {
	repo  *repository.AgentRepository
	redis *repository.RedisClient
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
		if repository.IsUniqueViolation(err) {
			return ErrAgentAlreadyExists
		}
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

func (s *AgentService) GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*model.Agent, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}

	agent, err := s.repo.GetByIDForTenant(ctx, tenantID, id)
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

func (s *AgentService) UpdateForTenant(ctx context.Context, tenantID uuid.UUID, agent *model.Agent) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	if agent == nil {
		return ErrInvalidMessage
	}

	agent.UpdatedAt = time.Now()
	updated, err := s.repo.UpdateForTenant(ctx, tenantID, agent)
	if err != nil {
		return err
	}
	if !updated {
		return ErrAgentNotFound
	}
	return nil
}

func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	return s.repo.Delete(ctx, id)
}

func (s *AgentService) DeleteForTenant(ctx context.Context, tenantID, id uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	deleted, err := s.repo.DeleteForTenant(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrAgentNotFound
	}
	return nil
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

func (s *AgentService) HeartbeatForTenant(ctx context.Context, tenantID, id uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	updated, err := s.repo.UpdateStatusForTenant(ctx, tenantID, id, model.AgentStatusOnline)
	if err != nil {
		return err
	}
	if !updated {
		return ErrAgentNotFound
	}
	return nil
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
	start := time.Now()
	ctx, span := otel.Tracer("agentmsg/service").Start(ctx, "message.send")
	defer span.End()
	if s == nil || s.repo == nil || s.redis == nil {
		observability.RecordMessageOperation("send", "service_unavailable", time.Since(start))
		span.SetAttributes(attribute.String("message.outcome", "service_unavailable"))
		return nil, ErrServiceUnavailable
	}
	if msg == nil || len(msg.RecipientIDs) == 0 || len(msg.Content) == 0 {
		observability.RecordMessageOperation("send", "invalid_message", time.Since(start))
		span.SetAttributes(attribute.String("message.outcome", "invalid_message"))
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
		observability.RecordMessageOperation("send", "recipient_encode_error", time.Since(start))
		span.RecordError(err)
		return nil, err
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		observability.RecordMessageOperation("send", "db_error", time.Since(start))
		span.RecordError(err)
		return nil, err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		observability.RecordMessageOperation("send", "marshal_error", time.Since(start))
		span.RecordError(err)
		return nil, err
	}

	if err := s.redis.LPush(ctx, "message:pending", string(payload)); err != nil {
		observability.RecordMessageOperation("send", "queue_error", time.Since(start))
		span.RecordError(err)
		return nil, err
	}

	observability.RecordMessageOperation("send", "queued", time.Since(start))
	span.SetAttributes(
		attribute.String("message.id", msg.ID.String()),
		attribute.String("message.trace_id", msg.TraceID),
		attribute.Int("message.recipient_count", len(msg.RecipientIDs)),
		attribute.String("message.outcome", "queued"),
	)

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

func (s *MessageService) GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	msg, err := s.repo.GetByIDForTenant(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, ErrMessageNotFound
	}
	return msg, nil
}

func (s *MessageService) Acknowledge(ctx context.Context, ack *model.Acknowledgement) error {
	start := time.Now()
	ctx, span := otel.Tracer("agentmsg/service").Start(ctx, "message.acknowledge")
	defer span.End()
	if s == nil || s.repo == nil || s.ackRepo == nil {
		observability.RecordMessageOperation("ack", "service_unavailable", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "service_unavailable"))
		return ErrServiceUnavailable
	}
	if ack == nil {
		observability.RecordMessageOperation("ack", "invalid_message", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "invalid_message"))
		return ErrInvalidMessage
	}
	if ack.MessageID == uuid.Nil || ack.AgentID == uuid.Nil || ack.Status == "" {
		observability.RecordMessageOperation("ack", "invalid_message", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "invalid_message"))
		return ErrInvalidMessage
	}

	msg, err := s.repo.GetByID(ctx, ack.MessageID)
	if err != nil {
		observability.RecordMessageOperation("ack", "lookup_error", time.Since(start))
		span.RecordError(err)
		return err
	}
	if msg == nil {
		observability.RecordMessageOperation("ack", "message_not_found", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "message_not_found"))
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
		observability.RecordMessageOperation("ack", "persist_error", time.Since(start))
		span.RecordError(err)
		return err
	}

	if err := s.repo.UpdateStatus(ctx, ack.MessageID, mapAckToMessageStatus(ack.Status)); err != nil {
		observability.RecordMessageOperation("ack", "status_update_error", time.Since(start))
		span.RecordError(err)
		return err
	}

	observability.RecordMessageOperation("ack", string(ack.Status), time.Since(start))
	span.SetAttributes(
		attribute.String("message.id", ack.MessageID.String()),
		attribute.String("ack.status", string(ack.Status)),
		attribute.String("ack.outcome", string(ack.Status)),
	)
	return nil
}

func (s *MessageService) AcknowledgeForTenant(ctx context.Context, tenantID uuid.UUID, ack *model.Acknowledgement) error {
	start := time.Now()
	ctx, span := otel.Tracer("agentmsg/service").Start(ctx, "message.acknowledge")
	defer span.End()
	if s == nil || s.repo == nil || s.ackRepo == nil {
		observability.RecordMessageOperation("ack", "service_unavailable", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "service_unavailable"))
		return ErrServiceUnavailable
	}
	if ack == nil {
		observability.RecordMessageOperation("ack", "invalid_message", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "invalid_message"))
		return ErrInvalidMessage
	}
	if ack.MessageID == uuid.Nil || ack.AgentID == uuid.Nil || ack.Status == "" {
		observability.RecordMessageOperation("ack", "invalid_message", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "invalid_message"))
		return ErrInvalidMessage
	}

	msg, err := s.repo.GetByIDForTenant(ctx, tenantID, ack.MessageID)
	if err != nil {
		observability.RecordMessageOperation("ack", "lookup_error", time.Since(start))
		span.RecordError(err)
		return err
	}
	if msg == nil {
		observability.RecordMessageOperation("ack", "message_not_found", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "message_not_found"))
		return ErrMessageNotFound
	}
	if !messageHasRecipient(msg, ack.AgentID) {
		observability.RecordMessageOperation("ack", "unauthorized", time.Since(start))
		span.SetAttributes(attribute.String("ack.outcome", "unauthorized"))
		return ErrUnauthorized
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
		observability.RecordMessageOperation("ack", "persist_error", time.Since(start))
		span.RecordError(err)
		return err
	}

	if err := s.repo.UpdateStatus(ctx, ack.MessageID, mapAckToMessageStatus(ack.Status)); err != nil {
		observability.RecordMessageOperation("ack", "status_update_error", time.Since(start))
		span.RecordError(err)
		return err
	}

	observability.RecordMessageOperation("ack", string(ack.Status), time.Since(start))
	span.SetAttributes(
		attribute.String("message.id", ack.MessageID.String()),
		attribute.String("ack.status", string(ack.Status)),
		attribute.String("ack.outcome", string(ack.Status)),
	)
	return nil
}

func (s *MessageService) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListByConversation(ctx, conversationID, limit)
}

func (s *MessageService) ListByConversationForTenant(ctx context.Context, tenantID, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if s == nil || s.repo == nil {
		return nil, ErrServiceUnavailable
	}
	return s.repo.ListByConversationForTenant(ctx, tenantID, conversationID, limit)
}

type AuthService struct {
	jwtSecret []byte
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub       string `json:"sub"`
	AgentID   string `json:"agent_id"`
	TenantID  string `json:"tenant_id"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
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

func messageHasRecipient(msg *model.Message, agentID uuid.UUID) bool {
	if msg == nil {
		return false
	}
	for _, recipientID := range msg.RecipientIDs {
		if recipientID == agentID {
			return true
		}
	}
	return false
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
