package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"agentmsg/internal/model"
)

func TestAgentServiceRegisterRequiresDependencies(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agent := &model.Agent{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		DID:       "did:agent:test-001",
		PublicKey: "test-key",
		Name:      "Test Agent",
	}

	err := svc.Register(ctx, agent)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestAgentServiceGetByIDWithoutRepository(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{
		repo: nil,
	}

	agentID := uuid.New()

	agent, err := svc.GetByID(ctx, agentID)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
	assert.Nil(t, agent)
}

func TestAgentServiceHeartbeatRequiresDependencies(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agentID := uuid.New()

	err := svc.Heartbeat(ctx, agentID)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestAgentServiceUpdateCapabilitiesWithoutRepository(t *testing.T) {
	ctx := context.Background()

	svc := &AgentService{}

	agentID := uuid.New()
	capabilities := model.Capabilities{
		{
			Type:        model.CapabilityTextGeneration,
			Description: "Generate text",
		},
	}

	err := svc.UpdateCapabilities(ctx, agentID, capabilities)
	assert.ErrorIs(t, err, ErrServiceUnavailable)
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

	agentID := uuid.New()
	tenantID := uuid.New()

	token, err := svc.GenerateToken(agentID, tenantID)
	assert.NoError(t, err)

	claims, err := svc.ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, agentID, claims.AgentID)
	assert.Equal(t, tenantID, claims.TenantID)
}

func TestAuthServiceValidateTokenRejectsInvalidInput(t *testing.T) {
	svc := &AuthService{
		jwtSecret: []byte("test-secret"),
	}

	claims, err := svc.ValidateToken("some-token")
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, claims)
}
