-- Migration: Add billing and idempotency tables
-- Version: 002
-- Description: Add billing events, invoices, pricing plans, and idempotency keys

-- Create billing_events table
CREATE TABLE IF NOT EXISTS billing_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    agent_id UUID REFERENCES agents(id),
    event_type VARCHAR(50) NOT NULL,
    message_id UUID REFERENCES messages(id),
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(10, 6) NOT NULL DEFAULT 0,
    total_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'USD',
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create invoices table
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    invoice_number VARCHAR(100) UNIQUE NOT NULL,
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    subtotal DECIMAL(10, 2) NOT NULL DEFAULT 0,
    tax DECIMAL(10, 2) NOT NULL DEFAULT 0,
    total DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'USD',
    status VARCHAR(50) DEFAULT 'draft',
    due_date TIMESTAMP WITH TIME ZONE NOT NULL,
    paid_at TIMESTAMP WITH TIME ZONE,
    line_items JSONB DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create pricing_plans table
CREATE TABLE IF NOT EXISTS pricing_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    plan_type VARCHAR(50) NOT NULL,
    price_monthly DECIMAL(10, 2) NOT NULL DEFAULT 0,
    price_yearly DECIMAL(10, 2) NOT NULL DEFAULT 0,
    message_quota BIGINT NOT NULL DEFAULT 0,
    agent_limit INT NOT NULL DEFAULT 1,
    features JSONB DEFAULT '[]',
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create idempotency_keys table for exactly-once delivery
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key_hash VARCHAR(64) PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES messages(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Create dead_letter_queue table
CREATE TABLE IF NOT EXISTS dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id),
    reason TEXT NOT NULL,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    payload JSONB NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Insert default pricing plans
INSERT INTO pricing_plans (id, name, plan_type, price_monthly, price_yearly, message_quota, agent_limit, features, status)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'Free', 'free', 0, 0, 10000, 5, '["basic_messaging", "capability_discovery"]', 'active'),
    ('00000000-0000-0000-0000-000000000002', 'Starter', 'starter', 29, 290, 100000, 20, '["basic_messaging", "capability_discovery", "webhooks", "priority_support"]', 'active'),
    ('00000000-0000-0000-0000-000000000003', 'Professional', 'professional', 99, 990, 1000000, 100, '["basic_messaging", "capability_discovery", "webhooks", "priority_support", "advanced_analytics", "sla_guarantee"]', 'active'),
    ('00000000-0000-0000-0000-000000000004', 'Enterprise', 'enterprise', 399, 3990, -1, -1, '["all_features", "dedicated_support", "custom_sla", "sso", "audit_logs"]', 'active')
ON CONFLICT (id) DO NOTHING;

-- Create indexes for new tables
CREATE INDEX IF NOT EXISTS idx_billing_events_tenant_id ON billing_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_events_created_at ON billing_events(created_at);
CREATE INDEX IF NOT EXISTS idx_billing_events_period ON billing_events(period_start, period_end);

CREATE INDEX IF NOT EXISTS idx_invoices_tenant_id ON invoices(tenant_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoices_due_date ON invoices(due_date);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);

CREATE INDEX IF NOT EXISTS idx_dlq_status ON dead_letter_queue(status);
CREATE INDEX IF NOT EXISTS idx_dlq_created_at ON dead_letter_queue(created_at);
