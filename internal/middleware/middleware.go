package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"agentmsg/internal/model"
	"agentmsg/internal/repository"
	"agentmsg/internal/service"
)

type Middleware struct {
	redis              *repository.RedisClient
	db                 *repository.PostgresDB
	auth               *service.AuthService
	rateLimitRequests  int
	rateLimitWindow    time.Duration
}

func NewMiddleware(redis *repository.RedisClient, db *repository.PostgresDB, auth *service.AuthService, rateLimitRequests int, rateLimitWindow time.Duration) *Middleware {
	if rateLimitRequests <= 0 {
		rateLimitRequests = 600
	}
	if rateLimitWindow <= 0 {
		rateLimitWindow = time.Minute
	}

	return &Middleware{
		redis:             redis,
		db:                db,
		auth:              auth,
		rateLimitRequests: rateLimitRequests,
		rateLimitWindow:   rateLimitWindow,
	}
}

func (m *Middleware) Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = requestID
		}

		c.Set("request_id", requestID)
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Trace-ID", traceID)

		c.Next()
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
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		slog.Info("request completed",
			"request_id", c.GetString("request_id"),
			"trace_id", c.GetString("trace_id"),
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"agent_id", c.GetString("agent_id"),
			"tenant_id", c.GetString("tenant_id"),
		)
	}
}

func (m *Middleware) AuditLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if m.db == nil || !isAuditMethod(c.Request.Method) {
			return
		}

		tenantID := parseOptionalUUID(c.GetString("tenant_id"))
		agentID := parseOptionalUUID(c.GetString("agent_id"))
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		entry := &model.AuditLog{
			ID:           uuid.New(),
			TenantID:     tenantID,
			AgentID:      agentID,
			RequestID:    c.GetString("request_id"),
			TraceID:      c.GetString("trace_id"),
			Action:       auditAction(c.Request.Method, path),
			ResourceType: auditResourceType(path),
			ResourceID:   c.Param("id"),
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			StatusCode:   c.Writer.Status(),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Metadata: model.AuditMetadata{
				"route":  path,
				"status": c.Writer.Status(),
			},
			CreatedAt: time.Now(),
		}

		if err := m.db.CreateAuditLog(c.Request.Context(), entry); err != nil {
			slog.Error("failed to persist audit log", "request_id", entry.RequestID, "error", err)
		}
	}
}

func isAuditMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func auditAction(method, path string) string {
	resource := auditResourceType(path)
	switch method {
	case http.MethodPost:
		if strings.HasSuffix(path, "/ack") {
			return resource + ".ack"
		}
		return resource + ".create"
	case http.MethodPut, http.MethodPatch:
		return resource + ".update"
	case http.MethodDelete:
		return resource + ".delete"
	default:
		return resource + ".access"
	}
}

func auditResourceType(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

func parseOptionalUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}
