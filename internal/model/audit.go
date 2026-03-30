package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditMetadata map[string]any

func (m AuditMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *AuditMetadata) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		*m = AuditMetadata{}
		return nil
	}
}

type AuditLog struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	TenantID    *uuid.UUID    `json:"tenantId,omitempty" db:"tenant_id"`
	AgentID     *uuid.UUID    `json:"agentId,omitempty" db:"agent_id"`
	RequestID   string        `json:"requestId" db:"request_id"`
	TraceID     string        `json:"traceId" db:"trace_id"`
	Action      string        `json:"action" db:"action"`
	ResourceType string       `json:"resourceType" db:"resource_type"`
	ResourceID  string        `json:"resourceId,omitempty" db:"resource_id"`
	Method      string        `json:"method" db:"method"`
	Path        string        `json:"path" db:"path"`
	StatusCode  int           `json:"statusCode" db:"status_code"`
	ClientIP    string        `json:"clientIp,omitempty" db:"client_ip"`
	UserAgent   string        `json:"userAgent,omitempty" db:"user_agent"`
	Metadata    AuditMetadata `json:"metadata" db:"metadata"`
	CreatedAt   time.Time     `json:"createdAt" db:"created_at"`
}
