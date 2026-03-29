package agentmsg

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Agent struct {
	ID            uuid.UUID    `json:"id"`
	TenantID      uuid.UUID    `json:"tenantId"`
	DID           string       `json:"did"`
	PublicKey     string       `json:"publicKey"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Provider      string       `json:"provider"`
	Tier          string       `json:"tier"`
	Capabilities  Capabilities `json:"capabilities"`
	Endpoints     Endpoints    `json:"endpoints"`
	TrustLevel    int          `json:"trustLevel"`
	VerifiedAt    *time.Time   `json:"verifiedAt,omitempty"`
	Status        AgentStatus  `json:"status"`
	LastHeartbeat time.Time    `json:"lastHeartbeat"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusAway   AgentStatus = "away"
	AgentStatusBusy   AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

type Capabilities []Capability

func (c Capabilities) Value() ([]byte, error) {
	return json.Marshal(c)
}

func (c *Capabilities) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), c)
}

type Capability struct {
	Type        CapabilityType    `json:"type"`
	Description string            `json:"description"`
	Examples    []string           `json:"examples,omitempty"`
	Parameters  *CapabilityParams  `json:"parameters,omitempty"`
	Constraints *CapabilityConstraints `json:"constraints,omitempty"`
	Quality     *CapabilityQuality `json:"quality,omitempty"`
}

type CapabilityType string

const (
	CapabilityTextGeneration  CapabilityType = "text-generation"
	CapabilityCodeGeneration  CapabilityType = "code-generation"
	CapabilityImageGeneration CapabilityType = "image-generation"
	CapabilityReasoning      CapabilityType = "reasoning"
	CapabilitySearch         CapabilityType = "search"
	CapabilityDataAnalysis   CapabilityType = "data-analysis"
)

type CapabilityParams struct {
	InputFormat   string `json:"inputFormat"`
	OutputFormat  string `json:"outputFormat"`
	MaxDurationMs int    `json:"maxDurationMs,omitempty"`
	MaxTokens     int    `json:"maxTokens,omitempty"`
}

type CapabilityConstraints struct {
	RateLimit   int     `json:"rateLimit,omitempty"`
	CostPerCall float64 `json:"costPerCall,omitempty"`
	Quota       int64   `json:"quota,omitempty"`
	QuotaUsed   int64   `json:"quotaUsed,omitempty"`
}

type CapabilityQuality struct {
	SuccessRate   float64 `json:"successRate,omitempty"`
	AvgLatencyMs  int     `json:"avgLatencyMs,omitempty"`
	Rating        float64 `json:"rating,omitempty"`
}

type Endpoints []Endpoint

func (e Endpoints) Value() ([]byte, error) {
	return json.Marshal(e)
}

func (e *Endpoints) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), e)
}

type Endpoint struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Weight    int    `json:"weight,omitempty"`
	IsPrimary bool   `json:"isPrimary"`
}