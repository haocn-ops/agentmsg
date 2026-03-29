package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"agentmsg/internal/repository"
)

type Middleware struct {
	redis *repository.RedisClient
}

func NewMiddleware(redis *repository.RedisClient) *Middleware {
	return &Middleware{redis: redis}
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

		c.Set("agent_id", "550e8400-e29b-41d4-a716-446655440000")
		c.Set("tenant_id", "550e8400-e29b-41d4-a716-446655440001")

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
