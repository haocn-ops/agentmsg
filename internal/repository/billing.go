package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"agentmsg/internal/model"
)

type BillingRepository struct {
	db *sqlx.DB
}

func NewBillingRepository(db *PostgresDB) *BillingRepository {
	return &BillingRepository{db: db.DB()}
}

func (r *BillingRepository) CreateEvent(ctx context.Context, event *model.BillingEvent) error {
	query := `
		INSERT INTO billing_events (id, tenant_id, agent_id, event_type, message_id, quantity, unit_price, total_amount, currency, period_start, period_end, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.TenantID, event.AgentID, event.EventType, event.MessageID,
		event.Quantity, event.UnitPrice, event.TotalAmount, event.Currency,
		event.PeriodStart, event.PeriodEnd, event.Status, event.CreatedAt,
	)
	return err
}

func (r *BillingRepository) CreateInvoice(ctx context.Context, invoice *model.Invoice) error {
	query := `
		INSERT INTO invoices (id, tenant_id, invoice_number, period_start, period_end, subtotal, tax, total, currency, status, due_date, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		invoice.ID, invoice.TenantID, invoice.InvoiceNumber, invoice.PeriodStart,
		invoice.PeriodEnd, invoice.Subtotal, invoice.Tax, invoice.Total,
		invoice.Currency, invoice.Status, invoice.DueDate, invoice.PaidAt, invoice.CreatedAt,
	)
	return err
}

func (r *BillingRepository) GetEventsInPeriod(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]model.BillingEvent, error) {
	var events []model.BillingEvent
	query := `SELECT * FROM billing_events WHERE tenant_id = $1 AND created_at >= $2 AND created_at <= $3`
	err := r.db.SelectContext(ctx, &events, query, tenantID, start, end)
	return events, err
}

func (r *BillingRepository) CountMessagesInPeriod(ctx context.Context, tenantID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM messages WHERE tenant_id = $1 AND created_at >= $2`
	err := r.db.GetContext(ctx, &count, query, tenantID, since)
	return count, err
}

func (r *BillingRepository) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	query := `SELECT * FROM tenants WHERE id = $1`
	err := r.db.GetContext(ctx, &tenant, query, tenantID)
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

type PlanRepository struct {
	db *sqlx.DB
}

func NewPlanRepository(db *PostgresDB) *PlanRepository {
	return &PlanRepository{db: db.DB()}
}

func (r *PlanRepository) ListActivePlans(ctx context.Context) ([]model.PricingPlan, error) {
	var plans []model.PricingPlan
	query := `SELECT * FROM pricing_plans WHERE status = 'active' ORDER BY price_monthly ASC`
	err := r.db.SelectContext(ctx, &plans, query)
	return plans, err
}

func (r *PlanRepository) GetPlanByID(ctx context.Context, id uuid.UUID) (*model.PricingPlan, error) {
	var plan model.PricingPlan
	query := `SELECT * FROM pricing_plans WHERE id = $1`
	err := r.db.GetContext(ctx, &plan, query, id)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}