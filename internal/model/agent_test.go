package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAgentCreation(t *testing.T) {
	agent := &Agent{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		DID:       "did:agent:test-001",
		PublicKey: "test-public-key",
		Name:      "Test Agent",
		Version:   "1.0.0",
		Provider:  "test",
		Tier:      "free",
		Status:    AgentStatusOnline,
	}

	assert.NotEmpty(t, agent.ID)
	assert.Equal(t, "did:agent:test-001", agent.DID)
	assert.Equal(t, "Test Agent", agent.Name)
	assert.Equal(t, AgentStatusOnline, agent.Status)
}

func TestCapabilitiesSerialization(t *testing.T) {
	capabilities := Capabilities{
		{
			Type:        CapabilityTextGeneration,
			Description: "Generate text",
			Parameters: &CapabilityParams{
				InputFormat:  "json",
				OutputFormat: "json",
			},
			Constraints: &CapabilityConstraints{
				RateLimit: 100,
			},
		},
	}

	data, err := capabilities.Value()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var result Capabilities
	err = result.Scan(data)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, CapabilityTextGeneration, result[0].Type)
}

func TestAgentStatusTransitions(t *testing.T) {
	agent := &Agent{
		ID:     uuid.New(),
		Status: AgentStatusOnline,
	}

	agent.Status = AgentStatusAway
	assert.Equal(t, AgentStatusAway, agent.Status)

	agent.Status = AgentStatusBusy
	assert.Equal(t, AgentStatusBusy, agent.Status)

	agent.Status = AgentStatusOffline
	assert.Equal(t, AgentStatusOffline, agent.Status)
}

func TestAgentJSONSerialization(t *testing.T) {
	agent := &Agent{
		ID:           uuid.New(),
		TenantID:     uuid.New(),
		DID:          "did:agent:test",
		PublicKey:    "key",
		Name:         "Test",
		Status:       AgentStatusOnline,
		Capabilities: []Capability{},
		Endpoints:    []Endpoint{},
	}

	data, err := json.Marshal(agent)
	assert.NoError(t, err)

	var result Agent
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, agent.DID, result.DID)
	assert.Equal(t, agent.Status, result.Status)
}

func TestCapabilityTypes(t *testing.T) {
	tests := []struct {
		capType CapabilityType
		want    string
	}{
		{CapabilityTextGeneration, "text-generation"},
		{CapabilityCodeGeneration, "code-generation"},
		{CapabilityImageGeneration, "image-generation"},
		{CapabilityReasoning, "reasoning"},
		{CapabilitySearch, "search"},
		{CapabilityDataAnalysis, "data-analysis"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.capType))
		})
	}
}

func TestEndpointsSerialization(t *testing.T) {
	endpoints := Endpoints{
		{
			Type:      "websocket",
			URL:       "wss://agent.example.com/ws",
			IsPrimary: true,
			Weight:    100,
		},
	}

	data, err := endpoints.Value()
	assert.NoError(t, err)

	var result Endpoints
	err = result.Scan(data)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "websocket", result[0].Type)
	assert.True(t, result[0].IsPrimary)
}
