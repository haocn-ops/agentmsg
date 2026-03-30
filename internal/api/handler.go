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
	config      *ServerConfig
	deps        *Dependencies
	httpServer  *http.Server
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
	AuthService   *service.AuthService
	Middleware     *middleware.Middleware
}

func NewServer(cfg *ServerConfig, deps *Dependencies) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

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
	v1.Use(s.deps.Middleware.Authenticate())
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
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

type AgentRequest struct {
	DID          string                `json:"did"`
	PublicKey    string               `json:"publicKey"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Provider     string              `json:"provider"`
	Capabilities []model.Capability   `json:"capabilities"`
}

func (s *Server) createAgent(c *gin.Context) {
	var req AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := uuid.MustParse(c.GetString("tenant_id"))

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, agent)
}

func (s *Server) getAgent(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	agent, err := s.deps.AgentService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (s *Server) listAgents(c *gin.Context) {
	tenantID := uuid.MustParse(c.GetString("tenant_id"))
	agents, err := s.deps.AgentService.ListByTenant(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

func (s *Server) updateAgent(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	var req AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, err := s.deps.AgentService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	agent.Name = req.Name
	agent.Version = req.Version
	agent.Capabilities = req.Capabilities

	if err := s.deps.AgentService.Update(c.Request.Context(), agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}

func (s *Server) deleteAgent(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	if err := s.deps.AgentService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) heartbeat(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	if err := s.deps.AgentService.Heartbeat(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type MessageRequest struct {
	MessageType   model.MessageType       `json:"messageType"`
	Recipients    []uuid.UUID             `json:"recipients"`
	Content       interface{}             `json:"content"`
	Metadata      map[string]interface{} `json:"metadata"`
	DeliveryGuarantee model.DeliveryGuarantee `json:"deliveryGuarantee"`
	TaskContext   *model.TaskContext     `json:"taskContext,omitempty"`
}

func buildMessageFromRequest(senderID, tenantID uuid.UUID, req MessageRequest) (*model.Message, error) {
	content, contentType, err := serializeContent(req.Content)
	if err != nil {
		return nil, err
	}

	deliveryGuarantee := req.DeliveryGuarantee
	if deliveryGuarantee == "" {
		deliveryGuarantee = model.DeliveryAtLeastOnce
	}

	return &model.Message{
		ConversationID:     uuid.New(),
		MessageType:        req.MessageType,
		SenderID:           senderID,
		RecipientIDs:       req.Recipients,
		Content:            content,
		ContentSize:        len(content),
		ContentType:        contentType,
		DeliveryGuarantee:  deliveryGuarantee,
		Metadata:           model.MessageMetadata{Custom: req.Metadata},
		TaskContext:        req.TaskContext,
		TenantID:           tenantID,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderID := uuid.MustParse(c.GetString("agent_id"))
	tenantID := uuid.MustParse(c.GetString("tenant_id"))

	msg, err := buildMessageFromRequest(senderID, tenantID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := s.deps.MessageService.Send(c.Request.Context(), msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (s *Server) getMessage(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	msg, err := s.deps.MessageService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}
	c.JSON(http.StatusOK, msg)
}

func (s *Server) acknowledgeMessage(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := model.MessageStatus(req.Status)
	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	if err := s.deps.MessageService.Acknowledge(c.Request.Context(), id, model.MessageStatus(status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

func (s *Server) getMessageStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
