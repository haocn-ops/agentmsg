package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"agentmsg/internal/model"
)

type PostgresDB struct {
	db *sqlx.DB
}

func (p *PostgresDB) GetMessageByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var msg model.Message
	query := `SELECT * FROM messages WHERE id = $1`
	err := p.db.GetContext(ctx, &msg, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := msg.ScanRecipients(); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (p *PostgresDB) CreateAcknowledgement(ctx context.Context, ack *model.Acknowledgement) error {
	query := `
		INSERT INTO acknowledgements (id, message_id, agent_id, status, details, nonce, signature, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := p.db.ExecContext(ctx, query,
		ack.ID, ack.MessageID, ack.AgentID, ack.Status, ack.Details, ack.Nonce, ack.Signature, ack.CreatedAt,
	)
	return err
}

func (p *PostgresDB) GetAcknowledgement(ctx context.Context, messageID uuid.UUID) (*model.Acknowledgement, error) {
	var ack model.Acknowledgement
	query := `SELECT * FROM acknowledgements WHERE message_id = $1 LIMIT 1`
	err := p.db.GetContext(ctx, &ack, query, messageID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ack, nil
}

func (p *PostgresDB) UpdateMessageStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error {
	query := `
		UPDATE messages
		SET
			status = $2,
			processed_at = CASE
				WHEN $2 IN ('delivered', 'processed', 'failed', 'dead_letter') THEN NOW()
				ELSE processed_at
			END
		WHERE id = $1
	`
	_, err := p.db.ExecContext(ctx, query, id, status)
	return err
}

func (p *PostgresDB) CreateDeadLetterEntry(ctx context.Context, entry *model.DeadLetterEntry) error {
	query := `
		INSERT INTO dead_letter_queue (id, message_id, reason, retry_count, max_retries, payload, status, created_at, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := p.db.ExecContext(ctx, query,
		entry.ID, entry.MessageID, entry.Reason, entry.RetryCount, entry.MaxRetries,
		string(entry.Payload), entry.Status, entry.CreatedAt, entry.ProcessedAt,
	)
	return err
}

func (p *PostgresDB) ListRetryableDeadLetterEntries(ctx context.Context, limit int) ([]model.DeadLetterEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	var entries []model.DeadLetterEntry
	query := `
		SELECT id, message_id, reason, retry_count, max_retries, payload, status, created_at, processed_at
		FROM dead_letter_queue
		WHERE status = 'pending' AND retry_count < max_retries
		ORDER BY created_at ASC
		LIMIT $1
	`
	if err := p.db.SelectContext(ctx, &entries, query, limit); err != nil {
		return nil, err
	}
	return entries, nil
}

func (p *PostgresDB) UpdateDeadLetterEntry(ctx context.Context, id uuid.UUID, status model.DeadLetterStatus, retryCount int, processedAt *time.Time) error {
	query := `
		UPDATE dead_letter_queue
		SET status = $2, retry_count = $3, processed_at = $4
		WHERE id = $1
	`
	_, err := p.db.ExecContext(ctx, query, id, status, retryCount, processedAt)
	return err
}

func NewPostgresDB(dsn string) (*PostgresDB, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) Close() error {
	return p.db.Close()
}

func (p *PostgresDB) DB() *sqlx.DB {
	return p.db
}

func (p *PostgresDB) Ping(ctx context.Context) error {
	if p == nil || p.db == nil {
		return sql.ErrConnDone
	}
	return p.db.PingContext(ctx)
}

type AgentRepository struct {
	db *sqlx.DB
}

func NewAgentRepository(db *PostgresDB) *AgentRepository {
	return &AgentRepository{db: db.DB()}
}

func (r *AgentRepository) Create(ctx context.Context, agent *model.Agent) error {
	query := `
		INSERT INTO agents (id, tenant_id, did, public_key, name, version, provider, tier, capabilities, endpoints, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.TenantID, agent.DID, agent.PublicKey,
		agent.Name, agent.Version, agent.Provider, agent.Tier,
		agent.Capabilities, agent.Endpoints, agent.Status,
	)
	return err
}

func (r *AgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Agent, error) {
	var agent model.Agent
	query := `SELECT * FROM agents WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &agent, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepository) GetByDID(ctx context.Context, did string) (*model.Agent, error) {
	var agent model.Agent
	query := `SELECT * FROM agents WHERE did = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &agent, query, did)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepository) Update(ctx context.Context, agent *model.Agent) error {
	query := `
		UPDATE agents SET
			name = $2, version = $3, capabilities = $4, endpoints = $5, status = $6, last_heartbeat = $7
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.Name, agent.Version,
		agent.Capabilities, agent.Endpoints, agent.Status, agent.LastHeartbeat,
	)
	return err
}

func (r *AgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE agents SET deleted_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *AgentRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.Agent, error) {
	var agents []model.Agent
	query := `SELECT * FROM agents WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &agents, query, tenantID)
	if err != nil {
		return nil, err
	}
	return agents, nil
}

func (r *AgentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AgentStatus) error {
	query := `UPDATE agents SET status = $2, last_heartbeat = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	return err
}

func (r *AgentRepository) QueryByCapabilities(ctx context.Context, tenantID uuid.UUID, capabilities []string) ([]model.Agent, error) {
	var agents []model.Agent
	query := `
		SELECT * FROM agents
		WHERE tenant_id = $1
		AND deleted_at IS NULL
		AND status = 'online'
		ORDER BY created_at DESC
		LIMIT 50
	`
	err := r.db.SelectContext(ctx, &agents, query, tenantID)
	if err != nil {
		return nil, err
	}
	return agents, nil
}

func (r *AgentRepository) ListAll(ctx context.Context) ([]model.Agent, error) {
	var agents []model.Agent
	query := `SELECT * FROM agents WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 1000`
	err := r.db.SelectContext(ctx, &agents, query)
	if err != nil {
		return nil, err
	}
	return agents, nil
}

func (r *AgentRepository) GetByStatus(ctx context.Context, status model.AgentStatus) ([]model.Agent, error) {
	var agents []model.Agent
	query := `SELECT * FROM agents WHERE status = $1 AND deleted_at IS NULL ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &agents, query, status)
	if err != nil {
		return nil, err
	}
	return agents, nil
}

type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *PostgresDB) *MessageRepository {
	return &MessageRepository{db: db.DB()}
}

func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	if msg.RecipientStr == "" {
		if err := msg.SetRecipients(); err != nil {
			return err
		}
	}

	query := `
		INSERT INTO messages (id, conversation_id, message_type, sender_id, recipient_ids, content, content_size, content_type, metadata, delivery_guarantee, status, task_context, trace_id, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.ConversationID, msg.MessageType, msg.SenderID, msg.RecipientStr,
		msg.Content, msg.ContentSize, msg.ContentType, msg.Metadata, msg.DeliveryGuarantee,
		msg.Status, msg.TaskContext, msg.TraceID, msg.TenantID,
	)
	return err
}

func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var msg model.Message
	query := `SELECT * FROM messages WHERE id = $1`
	err := r.db.GetContext(ctx, &msg, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := msg.ScanRecipients(); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *MessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error {
	query := `
		UPDATE messages
		SET
			status = $2,
			processed_at = CASE
				WHEN $2 IN ('delivered', 'processed', 'failed', 'dead_letter') THEN NOW()
				ELSE processed_at
			END
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, status)
	return err
}

func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	var messages []model.Message
	query := `SELECT * FROM messages WHERE conversation_id = $1 ORDER BY created_at DESC LIMIT $2`
	err := r.db.SelectContext(ctx, &messages, query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		if err := messages[i].ScanRecipients(); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (r *MessageRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]model.Message, error) {
	var messages []model.Message
	query := `SELECT * FROM messages WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err := r.db.SelectContext(ctx, &messages, query, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		if err := messages[i].ScanRecipients(); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

type AcknowledgementRepository struct {
	db *sqlx.DB
}

func NewAcknowledgementRepository(db *PostgresDB) *AcknowledgementRepository {
	return &AcknowledgementRepository{db: db.DB()}
}

func (r *AcknowledgementRepository) Create(ctx context.Context, ack *model.Acknowledgement) error {
	query := `
		INSERT INTO acknowledgements (id, message_id, agent_id, status, details, nonce, signature, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		ack.ID, ack.MessageID, ack.AgentID, ack.Status, ack.Details, ack.Nonce, ack.Signature, ack.CreatedAt,
	)
	return err
}

func (r *AcknowledgementRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*model.Acknowledgement, error) {
	var ack model.Acknowledgement
	query := `SELECT * FROM acknowledgements WHERE message_id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &ack, query, messageID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ack, nil
}

type TenantRepository struct {
	db *sqlx.DB
}

func NewTenantRepository(db *PostgresDB) *TenantRepository {
	return &TenantRepository{db: db.DB()}
}

func (r *TenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	query := `
		INSERT INTO tenants (id, name, slug, plan, limits, usage, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		tenant.ID, tenant.Name, tenant.Slug, tenant.Plan, tenant.Limits, tenant.Usage, tenant.Status,
	)
	return err
}

func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	query := `SELECT * FROM tenants WHERE id = $1`
	err := r.db.GetContext(ctx, &tenant, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}
