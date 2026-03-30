package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"agentmsg/internal/model"
)

type BatchMessageRequest struct {
	Messages []MessageRequest `json:"messages"`
}

func (s *Server) sendBatchMessages(c *gin.Context) {
	var req BatchMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderID := uuid.MustParse(c.GetString("agent_id"))
	tenantID := uuid.MustParse(c.GetString("tenant_id"))

	results := make([]model.SendResult, 0, len(req.Messages))
	for _, msgReq := range req.Messages {
		msg, err := buildMessageFromRequest(senderID, tenantID, msgReq)
		if err != nil {
			results = append(results, model.SendResult{
				Status: "error: " + err.Error(),
			})
			continue
		}

		result, err := s.deps.MessageService.Send(c.Request.Context(), msg)
		if err != nil {
			results = append(results, model.SendResult{
				MessageID: msg.ID,
				Status:    "error: " + err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}

	c.JSON(http.StatusCreated, gin.H{"results": results})
}

type ListMessagesRequest struct {
	ConversationID string `form:"conversationId"`
	Limit          int    `form:"limit,default=100"`
	Offset         int    `form:"offset,default=0"`
}

func (s *Server) listMessages(c *gin.Context) {
	var req ListMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := uuid.MustParse(c.GetString("tenant_id"))

	var messages []model.Message
	var err error

	if req.ConversationID != "" {
		convID, parseErr := uuid.Parse(req.ConversationID)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversationId"})
			return
		}
		messages, err = s.deps.MessageService.ListByConversation(c.Request.Context(), convID, req.Limit)
	} else {
		messages, err = s.deps.MessageService.ListByTenant(c.Request.Context(), tenantID, req.Limit, req.Offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"count":    len(messages),
		"limit":    req.Limit,
		"offset":   req.Offset,
	})
}

type CreateSubscriptionRequest struct {
	Type   model.SubType              `json:"type"`
	Filter model.SubscriptionFilter   `json:"filter"`
}

func (s *Server) createSubscription(c *gin.Context) {
	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentID := uuid.MustParse(c.GetString("agent_id"))
	tenantID := uuid.MustParse(c.GetString("tenant_id"))

	subscription := &model.Subscription{
		ID:        uuid.New(),
		AgentID:   agentID,
		TenantID:  tenantID,
		Type:      req.Type,
		Filter:    req.Filter,
		Status:    model.SubStatusActive,
		CreatedAt: time.Now().Unix(),
	}

	if err := s.deps.MessageService.CreateSubscription(c.Request.Context(), subscription); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

func (s *Server) listSubscriptions(c *gin.Context) {
	agentID := uuid.MustParse(c.GetString("agent_id"))

	subscriptions, err := s.deps.MessageService.ListSubscriptions(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": subscriptions,
		"count":         len(subscriptions),
	})
}

func (s *Server) deleteSubscription(c *gin.Context) {
	id := uuid.MustParse(c.Param("id"))
	agentID := uuid.MustParse(c.GetString("agent_id"))

	if err := s.deps.MessageService.DeleteSubscription(c.Request.Context(), agentID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

type CapabilityQueryRequest struct {
	Capabilities []string           `json:"capabilities"`
	MessageTypes []model.MessageType `json:"messageTypes,omitempty"`
	Tags         map[string]string  `json:"tags,omitempty"`
}

func (s *Server) queryCapabilities(c *gin.Context) {
	var req CapabilityQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := uuid.MustParse(c.GetString("tenant_id"))

	agents, err := s.deps.AgentService.QueryByCapabilities(c.Request.Context(), tenantID, req.Capabilities)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents":  agents,
		"count":   len(agents),
	})
}
