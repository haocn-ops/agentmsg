package model

import "github.com/google/uuid"

type SendResult struct {
	MessageID   uuid.UUID `json:"messageId"`
	Status      string    `json:"status"`
	DeliveredAt *int64   `json:"deliveredAt,omitempty"`
}

type TokenClaims struct {
	AgentID   uuid.UUID
	TenantID  uuid.UUID
	IssuedAt  int64
	ExpiresAt int64
}

type Tenant struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Slug         string    `json:"slug" db:"slug"`
	Plan         string    `json:"plan" db:"plan"`
	Limits       TenantLimits `json:"limits" db:"limits"`
	Usage        TenantUsage  `json:"usage" db:"usage"`
	MessageUsed  int64     `json:"messageUsed" db:"message_used"`
	AgentCount   int        `json:"agentCount" db:"agent_count"`
	Status       string    `json:"status" db:"status"`
	SSOEnabled   bool      `json:"ssoEnabled" db:"sso_enabled"`
	BillingEmail string    `json:"billingEmail" db:"billing_email"`
}

type TenantLimits struct {
	MessagePerMonth int64  `json:"messagePerMonth"`
	MaxAgents      int    `json:"maxAgents"`
	MaxConnections int    `json:"maxConnections"`
	RetentionDays  int    `json:"retentionDays"`
	SlaPercent     float64 `json:"slaPercent"`
}

type TenantUsage struct {
	MessageCount   int64 `json:"messageCount"`
	BandwidthBytes int64 `json:"bandwidthBytes"`
	StorageBytes   int64 `json:"storageBytes"`
	ApiCalls       int64 `json:"apiCalls"`
	ResetAt        int64 `json:"resetAt"`
}

type Subscription struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	AgentID   uuid.UUID       `json:"agentId" db:"agent_id"`
	TenantID  uuid.UUID       `json:"tenantId" db:"tenant_id"`
	Type      SubType         `json:"type" db:"type"`
	Filter    SubscriptionFilter `json:"filter" db:"filter"`
	Status    SubStatus       `json:"status" db:"status"`
	CreatedAt int64          `json:"createdAt" db:"created_at"`
}

type SubType string

const (
	SubTypeDirect     SubType = "direct"
	SubTypeCapability SubType = "capability"
	SubTypeTopic      SubType = "topic"
	SubTypePattern    SubType = "pattern"
)

type SubStatus string

const (
	SubStatusActive    SubStatus = "active"
	SubStatusPaused    SubStatus = "paused"
	SubStatusCancelled SubStatus = "cancelled"
)

type SubscriptionFilter struct {
	AgentIDs         []uuid.UUID `json:"agentIds,omitempty"`
	CapabilityTypes  []string    `json:"capabilityTypes,omitempty"`
	Topics           []string    `json:"topics,omitempty"`
	MessageTypes     []string    `json:"messageTypes,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
}
