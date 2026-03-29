package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BillingEvent struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	TenantID    uuid.UUID       `json:"tenantId" db:"tenant_id"`
	AgentID     uuid.UUID       `json:"agentId" db:"agent_id"`
	EventType   BillingEventType `json:"eventType" db:"event_type"`
	MessageID   *uuid.UUID      `json:"messageId,omitempty" db:"message_id"`
	Quantity    int             `json:"quantity" db:"quantity"`
	UnitPrice   float64         `json:"unitPrice" db:"unit_price"`
	TotalAmount float64         `json:"totalAmount" db:"total_amount"`
	Currency    string          `json:"currency" db:"currency"`
	PeriodStart time.Time      `json:"periodStart" db:"period_start"`
	PeriodEnd   time.Time       `json:"periodEnd" db:"period_end"`
	Status      BillingStatus   `json:"status" db:"status"`
	CreatedAt   time.Time       `json:"createdAt" db:"created_at"`
}

type BillingEventType string

const (
	BillingEventMessageSent     BillingEventType = "message_sent"
	BillingEventMessageReceived BillingEventType = "message_received"
	BillingEventAPICall        BillingEventType = "api_call"
	BillingEventStorage        BillingEventType = "storage"
	BillingEventBandwidth      BillingEventType = "bandwidth"
	BillingEventSubscription   BillingEventType = "subscription"
)

type BillingStatus string

const (
	BillingStatusPending   BillingStatus = "pending"
	BillingStatusActive    BillingStatus = "active"
	BillingStatusProcessed BillingStatus = "processed"
	BillingStatusFailed    BillingStatus = "failed"
	BillingStatusRefunded  BillingStatus = "refunded"
)

type Invoice struct {
	ID              uuid.UUID        `json:"id" db:"id"`
	TenantID        uuid.UUID        `json:"tenantId" db:"tenant_id"`
	InvoiceNumber   string           `json:"invoiceNumber" db:"invoice_number"`
	PeriodStart     time.Time        `json:"periodStart" db:"period_start"`
	PeriodEnd       time.Time        `json:"periodEnd" db:"period_end"`
	Subtotal        float64          `json:"subtotal" db:"subtotal"`
	Tax             float64          `json:"tax" db:"tax"`
	Total           float64          `json:"total" db:"total"`
	Currency        string           `json:"currency" db:"currency"`
	Status          InvoiceStatus    `json:"status" db:"status"`
	DueDate         time.Time        `json:"dueDate" db:"due_date"`
	PaidAt          *time.Time       `json:"paidAt,omitempty" db:"paid_at"`
	LineItems       []InvoiceLineItem `json:"lineItems" db:"-"`
	CreatedAt       time.Time        `json:"createdAt" db:"created_at"`
}

type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusPending   InvoiceStatus = "pending"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusOverdue   InvoiceStatus = "overdue"
	InvoiceStatusCancelled InvoiceStatus = "cancelled"
)

type InvoiceLineItem struct {
	Description string  `json:"description" db:"description"`
	Quantity    int     `json:"quantity" db:"quantity"`
	UnitPrice   float64 `json:"unitPrice" db:"unit_price"`
	Amount      float64 `json:"amount" db:"amount"`
}

type PricingPlan struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	Name         string         `json:"name" db:"name"`
	PlanType     PlanType       `json:"planType" db:"plan_type"`
	PriceMonthly float64        `json:"priceMonthly" db:"price_monthly"`
	PriceYearly  float64        `json:"priceYearly" db:"price_yearly"`
	MessageQuota int64          `json:"messageQuota" db:"message_quota"`
	AgentLimit   int            `json:"agentLimit" db:"agent_limit"`
	Features     []string       `json:"features" db:"-"`
	Status       PlanStatus     `json:"status" db:"status"`
	CreatedAt    time.Time      `json:"createdAt" db:"created_at"`
}

type PlanType string

const (
	PlanTypeFree       PlanType = "free"
	PlanTypeStarter    PlanType = "starter"
	PlanTypeProfessional PlanType = "professional"
	PlanTypeEnterprise  PlanType = "enterprise"
)

type PlanStatus string

const (
	PlanStatusActive  PlanStatus = "active"
	PlanStatusDeprecated PlanStatus = "deprecated"
)

type UsageRecord struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    uuid.UUID  `json:"tenantId" db:"tenant_id"`
	Metric      string     `json:"metric" db:"metric"`
	Value       int64      `json:"value" db:"value"`
	Unit        string     `json:"unit" db:"unit"`
	Period      string     `json:"period" db:"period"`
	Timestamp   time.Time  `json:"timestamp" db:"timestamp"`
}

func (i Invoice) Value() (interface{}, error) {
	return json.Marshal(i)
}

func (i *Invoice) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), i)
}