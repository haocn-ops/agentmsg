package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"agentmsg/internal/repository"
	"agentmsg/internal/service"
)

type Middleware struct {
	redis              *repository.RedisClient
	auth               *service.AuthService
	rateLimitRequests  int
	rateLimitWindow    time.Duration
}

func NewMiddleware(redis *repository.RedisClient, auth *service.AuthService, rateLimitRequests int, rateLimitWindow time.Duration) *Middleware {
	if rateLimitRequests <= 0 {
		rateLimitRequests = 600
	}
	if rateLimitWindow <= 0 {
		rateLimitWindow = time.Minute
	}

	return &Middleware{
		redis:             redis,
		auth:              auth,
		rateLimitRequests: rateLimitRequests,
		rateLimitWindow:   rateLimitWindow,
	}
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
		if m.redis == nil {
			c.Next()
			return
		}

		agentID := c.GetString("agent_id")
		tenantID := c.GetString("tenant_id")
		if agentID == "" || tenantID == "" {
			c.Next()
			return
		}

		windowSeconds := int(m.rateLimitWindow.Seconds())
		if windowSeconds <= 0 {
			windowSeconds = 60
		}

		windowSlot := time.Now().Unix() / int64(windowSeconds)
		key := strings.Join([]string{
			"ratelimit",
			tenantID,
			agentID,
			c.FullPath(),
			strconv.FormatInt(windowSlot, 10),
		}, ":")

		count, err := m.redis.Incr(c.Request.Context(), key)
		if err == nil && count == 1 {
			_ = m.redis.Expire(c.Request.Context(), key, windowSeconds)
		}
		if err != nil {
			c.Next()
			return
		}

		remaining := m.rateLimitRequests - int(count)
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(m.rateLimitRequests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if int(count) > m.rateLimitRequests {
			c.Header("Retry-After", strconv.Itoa(windowSeconds))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": windowSeconds,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (m *Middleware) Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
