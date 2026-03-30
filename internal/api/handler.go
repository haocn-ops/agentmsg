package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"agentmsg/internal/middleware"
	"agentmsg/internal/model"
	"agentmsg/internal/service"
)

type Server struct {
	config     *ServerConfig
	deps       *Dependencies
	httpServer *http.Server
}

type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type Dependencies struct {
	AgentService   *service.AgentService
	MessageService *service.MessageService
	AuthService    *service.AuthService
	Database       interface{ Ping(context.Context) error }
	Redis          interface{ Ping(context.Context) error }
	Middleware     *middleware.Middleware
}

func NewServer(cfg *ServerConfig, deps *Dependencies) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), deps.Middleware.Tracing(), deps.Middleware.Logging())

	s := &Server{
		config: cfg,
		deps:   deps,
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}

	s.setupRoutes(router)
	return s
}

func (s *Server) setupRoutes(r *gin.Engine) {
	r.GET("/health", s.healthCheck)
	r.GET("/ready", s.readinessCheck)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/api/v1")
	v1.Use(s.deps.Middleware.Authenticate(), s.deps.Middleware.RateLimit(), s.deps.Middleware.AuditLog())
	{
		agents := v1.Group("/agents")
		{
			agents.POST("", s.createAgent)
			agents.GET("", s.listAgents)
			agents.GET("/:id", s.getAgent)
			agents.PUT("/:id", s.updateAgent)
			agents.DELETE("/:id", s.deleteAgent)
			agents.POST("/:id/heartbeat", s.heartbeat)
		}

		messages := v1.Group("/messages")
		{
			messages.POST("", s.sendMessage)
			messages.POST("/batch", s.sendBatchMessages)
			messages.GET("", s.listMessages)
			messages.GET("/:id", s.getMessage)
			messages.POST("/:id/ack", s.acknowledgeMessage)
		}

		subs := v1.Group("/subscriptions")
		{
			subs.POST("", s.createSubscription)
			subs.GET("", s.listSubscriptions)
			subs.DELETE("/:id", s.deleteSubscription)
		}

		discovery := v1.Group("/discovery")
		{
			discovery.POST("/query", s.queryCapabilities)
		}
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (s *Server) readinessCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{}

	if s.deps.Database == nil {
		checks["database"] = "unconfigured"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
		return
	}
	if err := s.deps.Database.Ping(ctx); err != nil {
		checks["database"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
		return
	}
	checks["database"] = "ok"

	if s.deps.Redis == nil {
		checks["redis"] = "unconfigured"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
		return
	}
	if err := s.deps.Redis.Ping(ctx); err != nil {
		checks["redis"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
		return
	}
	checks["redis"] = "ok"

	c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
}

type AgentRequest struct {
	DID          string             `json:"did"`
	PublicKey    string             `json:"publicKey"`
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Provider     string             `json:"provider"`
	Capabilities []model.Capability `json:"capabilities"`
}

func (s *Server) createAgent(c *gin.Context) {
	var req AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}

	agent := &model.Agent{
		TenantID:     tenantID,
		DID:          req.DID,
		PublicKey:    req.PublicKey,
		Name:         req.Name,
		Version:      req.Version,
		Provider:     req.Provider,
		Capabilities: req.Capabilities,
	}

	if err := s.deps.AgentService.Register(c.Request.Context(), agent); err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}

	c.JSON(http.StatusCreated, agent)
}

func (s *Server) getAgent(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}
	agent, err := s.deps.AgentService.GetByIDForTenant(c.Request.Context(), tenantID, id)
	if err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (s *Server) listAgents(c *gin.Context) {
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}
	agents, err := s.deps.AgentService.ListByTenant(c.Request.Context(), tenantID)
	if err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}
	c.JSON(http.StatusOK, agents)
}

func (s *Server) updateAgent(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}

	agent, err := s.deps.AgentService.GetByIDForTenant(c.Request.Context(), tenantID, id)
	if err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}

	agent.Name = req.Name
	agent.Version = req.Version
	agent.Capabilities = req.Capabilities

	if err := s.deps.AgentService.UpdateForTenant(c.Request.Context(), tenantID, agent); err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}

	c.JSON(http.StatusOK, agent)
}

func (s *Server) deleteAgent(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}
	if err := s.deps.AgentService.DeleteForTenant(c.Request.Context(), tenantID, id); err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) heartbeat(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}
	if err := s.deps.AgentService.HeartbeatForTenant(c.Request.Context(), tenantID, id); err != nil {
		respondServiceError(c, err, "agent_not_found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type MessageRequest struct {
	MessageType       model.MessageType       `json:"messageType"`
	Recipients        []uuid.UUID             `json:"recipients"`
	Content           interface{}             `json:"content"`
	Metadata          map[string]interface{}  `json:"metadata"`
	DeliveryGuarantee model.DeliveryGuarantee `json:"deliveryGuarantee"`
	TaskContext       *model.TaskContext      `json:"taskContext,omitempty"`
}

func buildMessageFromRequest(senderID, tenantID uuid.UUID, traceID string, req MessageRequest) (*model.Message, error) {
	content, contentType, err := serializeContent(req.Content)
	if err != nil {
		return nil, err
	}

	deliveryGuarantee := req.DeliveryGuarantee
	if deliveryGuarantee == "" {
		deliveryGuarantee = model.DeliveryAtLeastOnce
	}

	return &model.Message{
		ConversationID:    uuid.New(),
		MessageType:       req.MessageType,
		SenderID:          senderID,
		RecipientIDs:      req.Recipients,
		Content:           content,
		ContentSize:       len(content),
		ContentType:       contentType,
		DeliveryGuarantee: deliveryGuarantee,
		Metadata:          model.MessageMetadata{Custom: req.Metadata},
		TaskContext:       req.TaskContext,
		TraceID:           traceID,
		TenantID:          tenantID,
	}, nil
}

func serializeContent(content interface{}) ([]byte, string, error) {
	if content == nil {
		return nil, "", errors.New("content is required")
	}

	switch value := content.(type) {
	case string:
		return []byte(value), "text/plain; charset=utf-8", nil
	case []byte:
		return value, "application/octet-stream", nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, "", err
		}
		return data, "application/json", nil
	}
}

func (s *Server) sendMessage(c *gin.Context) {
	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	senderID, ok := contextUUID(c, "agent_id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}

	msg, err := buildMessageFromRequest(senderID, tenantID, c.GetString("trace_id"), req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.deps.MessageService.Send(c.Request.Context(), msg)
	if err != nil {
		respondServiceError(c, err, "message_not_found")
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (s *Server) getMessage(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}
	msg, err := s.deps.MessageService.GetByIDForTenant(c.Request.Context(), tenantID, id)
	if err != nil {
		respondServiceError(c, err, "message_not_found")
		return
	}
	c.JSON(http.StatusOK, msg)
}

func (s *Server) acknowledgeMessage(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status    string `json:"status"`
		Details   string `json:"details"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ackStatus := model.AckStatus(req.Status)
	if ackStatus == "" {
		respondError(c, http.StatusBadRequest, "invalid_request", "status is required")
		return
	}
	agentID, ok := contextUUID(c, "agent_id")
	if !ok {
		return
	}
	tenantID, ok := contextUUID(c, "tenant_id")
	if !ok {
		return
	}

	ack := &model.Acknowledgement{
		MessageID: id,
		AgentID:   agentID,
		Status:    ackStatus,
		Details:   req.Details,
		Nonce:     req.Nonce,
		Signature: req.Signature,
	}

	if err := s.deps.MessageService.AcknowledgeForTenant(c.Request.Context(), tenantID, ack); err != nil {
		respondServiceError(c, err, "message_not_found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "acknowledged",
		"ackStatus":     ack.Status,
		"messageStatus": serviceMessageStatusForAck(ack.Status),
		"nonce":         ack.Nonce,
	})
}

func (s *Server) getMessageStats(c *gin.Context) {
	respondError(c, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func serviceMessageStatusForAck(status model.AckStatus) model.MessageStatus {
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
