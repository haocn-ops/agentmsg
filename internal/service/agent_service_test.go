package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockAgentRepo struct {
	agents map[uuid.UUID]*mockAgent
}

type mockAgent struct {
	id        uuid.UUID
	tenantID  uuid.UUID
	did       string
	publicKey string
	name      string
	status    string
}

func TestAgentServiceRegister(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agent := &Agent{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		DID:       "did:agent:test-001",
		PublicKey: "test-key",
		Name:      "Test Agent",
	}

	err := svc.Register(ctx, agent)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, agent.ID)
	assert.Equal(t, AgentStatusOnline, agent.Status)
}

func TestAgentServiceGetByID(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{
		repo: nil,
	}

	agentID := uuid.New()

	agent, err := svc.GetByID(ctx, agentID)
	assert.Error(t, err)
	assert.Equal(t, ErrAgentNotFound, err)
}

func TestAgentServiceHeartbeat(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agentID := uuid.New()

	err := svc.Heartbeat(ctx, agentID)
	assert.NoError(t, err)
}

func TestAgentServiceUpdateCapabilities(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agentID := uuid.New()
	capabilities := Capabilities{
		{
			Type:        CapabilityTextGeneration,
			Description: "Generate text",
		},
	}

	err := svc.UpdateCapabilities(ctx, agentID, capabilities)
	assert.Error(t, err)
}

func TestAuthServiceGenerateToken(t *testing.T) {
	svc := &AuthService{
		jwtSecret: []byte("test-secret"),
	}

	agentID := uuid.New()
	tenantID := uuid.New()

	token, err := svc.GenerateToken(agentID, tenantID)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestAuthServiceValidateToken(t *testing.T) {
	svc := &AuthService{
		jwtSecret: []byte("test-secret"),
	}

	token, err := svc.ValidateToken("some-token")
	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEqual(t, uuid.Nil, token.AgentID)
	assert.NotEqual(t, uuid.Nil, token.TenantID)
}
