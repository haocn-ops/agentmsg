package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"agentmsg/internal/engine"
	"agentmsg/internal/middleware"
	"agentmsg/internal/model"
	"agentmsg/internal/repository"
	"agentmsg/internal/service"
)

func TestMessageAckAndAuditE2E(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if databaseURL == "" || redisURL == "" {
		t.Skip("DATABASE_URL and REDIS_URL must be set for e2e integration tests")
	}

	db, err := repository.NewPostgresDB(databaseURL)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	redisClient, err := repository.NewRedisClient(redisURL)
	require.NoError(t, err)
	defer func() { require.NoError(t, redisClient.Close()) }()

	applyTestMigrations(t, db)

	tenantID := uuid.New()
	senderID := uuid.New()
	recipientID := uuid.New()
	insertTenantAndAgents(t, db, tenantID, senderID, recipientID)
	defer cleanupTenantData(t, db, tenantID)

	agentRepo := repository.NewAgentRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	ackRepo := repository.NewAcknowledgementRepository(db)

	agentService := service.NewAgentService(agentRepo, redisClient)
	messageService := service.NewMessageService(messageRepo, ackRepo, redisClient)
	authService := service.NewAuthService("test-secret")
	server := NewServer(&ServerConfig{
		Addr:         "127.0.0.1:0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}, &Dependencies{
		AgentService:   agentService,
		MessageService: messageService,
		AuthService:    authService,
		Database:       db,
		Redis:          redisClient,
		Middleware:     middleware.NewMiddleware(redisClient, db, authService, 1000, time.Minute),
	})

	msgEngine := engine.NewMessageEngine(&engine.EngineConfig{
		WorkerCount:    1,
		BatchSize:      10,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     2,
		RetryBaseDelay: 10 * time.Millisecond,
	}, db, redisClient)
	require.NoError(t, msgEngine.Start(context.Background()))
	defer msgEngine.Stop()

	httpServer := httptest.NewServer(server.httpServer.Handler)
	defer httpServer.Close()

	token, err := authService.GenerateToken(senderID, tenantID)
	require.NoError(t, err)

	sendPayload := map[string]any{
		"messageType":       "generic",
		"recipients":        []string{recipientID.String()},
		"content":           map[string]any{"text": "hello from e2e"},
		"deliveryGuarantee": "at_least_once",
	}

	reqBody, err := json.Marshal(sendPayload)
	require.NoError(t, err)

	sendReq, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/messages", bytes.NewReader(reqBody))
	require.NoError(t, err)
	sendReq.Header.Set("Authorization", "Bearer "+token)
	sendReq.Header.Set("Content-Type", "application/json")
	sendReq.Header.Set("X-Trace-ID", "trace-e2e")

	sendResp, err := http.DefaultClient.Do(sendReq)
	require.NoError(t, err)
	defer sendResp.Body.Close()
	require.Equal(t, http.StatusCreated, sendResp.StatusCode)

	var sendResult model.SendResult
	require.NoError(t, json.NewDecoder(sendResp.Body).Decode(&sendResult))
	require.NotEqual(t, uuid.Nil, sendResult.MessageID)
	require.Equal(t, string(model.MessageStatusPending), sendResult.Status)

	require.Eventually(t, func() bool {
		msg, getErr := db.GetMessageByID(context.Background(), sendResult.MessageID)
		return getErr == nil && msg != nil && msg.Status == model.MessageStatusSent && msg.TraceID == "trace-e2e"
	}, 5*time.Second, 100*time.Millisecond)

	ackPayload := map[string]any{
		"status":  "processed",
		"details": "completed in e2e test",
	}
	ackBody, err := json.Marshal(ackPayload)
	require.NoError(t, err)

	ackReq, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/messages/"+sendResult.MessageID.String()+"/ack", bytes.NewReader(ackBody))
	require.NoError(t, err)
	ackReq.Header.Set("Authorization", "Bearer "+token)
	ackReq.Header.Set("Content-Type", "application/json")
	ackReq.Header.Set("X-Trace-ID", "trace-e2e")

	ackResp, err := http.DefaultClient.Do(ackReq)
	require.NoError(t, err)
	defer ackResp.Body.Close()
	require.Equal(t, http.StatusOK, ackResp.StatusCode)

	require.Eventually(t, func() bool {
		msg, getErr := db.GetMessageByID(context.Background(), sendResult.MessageID)
		if getErr != nil || msg == nil || msg.Status != model.MessageStatusProcessed {
			return false
		}

		ack, ackErr := db.GetAcknowledgement(context.Background(), sendResult.MessageID)
		return ackErr == nil && ack != nil && ack.Status == model.AckStatusProcessed
	}, 5*time.Second, 100*time.Millisecond)

	var auditCount int
	query := `SELECT COUNT(*) FROM audit_logs WHERE trace_id = $1 AND action IN ('messages.create', 'messages.ack')`
	require.NoError(t, db.DB().GetContext(context.Background(), &auditCount, query, "trace-e2e"))
	require.GreaterOrEqual(t, auditCount, 2)
}

func applyTestMigrations(t *testing.T, db *repository.PostgresDB) {
	t.Helper()

	ctx := context.Background()
	_, err := db.DB().ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	require.NoError(t, err)

	migrationFiles := []string{
		"001_initial_schema.sql",
		"002_billing_schema.sql",
		"003_audit_logs.sql",
	}

	for _, file := range migrationFiles {
		path := filepath.Join("/Users/zh/Documents/codeX/agentmsg/internal/repository/migrations", file)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, execErr := db.DB().ExecContext(ctx, string(content))
		require.NoError(t, execErr)
	}
}

func insertTenantAndAgents(t *testing.T, db *repository.PostgresDB, tenantID, senderID, recipientID uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	_, err := db.DB().ExecContext(ctx, `
		INSERT INTO tenants (id, name, slug, plan, limits, usage, status)
		VALUES ($1, $2, $3, 'standard', '{}'::jsonb, '{}'::jsonb, 'active')
		ON CONFLICT (id) DO NOTHING
	`, tenantID, "E2E Tenant", "e2e-"+tenantID.String())
	require.NoError(t, err)

	insertAgent := func(agentID uuid.UUID, did string) {
		_, execErr := db.DB().ExecContext(ctx, `
			INSERT INTO agents (id, tenant_id, did, public_key, name, version, provider, tier, capabilities, endpoints, status)
			VALUES ($1, $2, $3, $4, $5, '1.0.0', 'e2e', 'free', '[]'::jsonb, '[]'::jsonb, 'online')
			ON CONFLICT (id) DO NOTHING
		`, agentID, tenantID, did, "test-public-key", did)
		require.NoError(t, execErr)
	}

	insertAgent(senderID, "did:agent:e2e:sender")
	insertAgent(recipientID, "did:agent:e2e:recipient")
}

func cleanupTenantData(t *testing.T, db *repository.PostgresDB, tenantID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var messageIDs []uuid.UUID
	require.NoError(t, db.DB().SelectContext(ctx, &messageIDs, `SELECT id FROM messages WHERE tenant_id = $1`, tenantID))

	_, _ = db.DB().ExecContext(ctx, `DELETE FROM audit_logs WHERE tenant_id = $1`, tenantID)

	if len(messageIDs) > 0 {
		query, args, err := sqlx.In(`DELETE FROM acknowledgements WHERE message_id IN (?)`, messageIDs)
		require.NoError(t, err)
		query = db.DB().Rebind(query)
		_, err = db.DB().ExecContext(ctx, query, args...)
		require.NoError(t, err)

		query, args, err = sqlx.In(`DELETE FROM dead_letter_queue WHERE message_id IN (?)`, messageIDs)
		require.NoError(t, err)
		query = db.DB().Rebind(query)
		_, err = db.DB().ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}

	_, _ = db.DB().ExecContext(ctx, `DELETE FROM messages WHERE tenant_id = $1`, tenantID)
	_, _ = db.DB().ExecContext(ctx, `DELETE FROM agents WHERE tenant_id = $1`, tenantID)
	_, _ = db.DB().ExecContext(ctx, `DELETE FROM audit_logs WHERE tenant_id = $1`, tenantID)
	_, _ = db.DB().ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
}
