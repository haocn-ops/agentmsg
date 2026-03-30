package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"agentmsg/internal/repository"
	"agentmsg/internal/service"
)

type Middleware struct {
	redis *repository.RedisClient
	auth  *service.AuthService
}

func NewMiddleware(redis *repository.RedisClient, auth *service.AuthService) *Middleware {
	return &Middleware{redis: redis, auth: auth}
}

func (m *Middleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		if m.auth == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication service unavailable"})
			c.Abort()
			return
		}

		claims, err := m.auth.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("agent_id", claims.AgentID.String())
		c.Set("tenant_id", claims.TenantID.String())

		c.Next()
	}
}

func (m *Middleware) TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (m *Middleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (m *Middleware) Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
