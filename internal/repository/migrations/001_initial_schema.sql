-- Migration: Create initial schema
-- Version: 001
-- Description: Initial schema for agents and messages

-- Create tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan VARCHAR(50) DEFAULT 'standard',
    limits JSONB NOT NULL DEFAULT '{}',
    usage JSONB NOT NULL DEFAULT '{}',
    message_used BIGINT DEFAULT 0,
    agent_count INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    sso_enabled BOOLEAN DEFAULT false,
    billing_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    did VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    name VARCHAR(255),
    version VARCHAR(50),
    provider VARCHAR(100),
    tier VARCHAR(50) DEFAULT 'free',
    capabilities JSONB NOT NULL DEFAULT '[]',
    endpoints JSONB DEFAULT '[]',
    trust_level INT DEFAULT 1,
    verified_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'offline',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create messages table
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_type VARCHAR(50) NOT NULL,
    sender_id UUID NOT NULL REFERENCES agents(id),
    recipient_ids TEXT NOT NULL,
    content BYTEA NOT NULL,
    content_size INT NOT NULL,
    content_type VARCHAR(100),
    metadata JSONB DEFAULT '{}',
    delivery_guarantee VARCHAR(50) DEFAULT 'at_least_once',
    status VARCHAR(50) DEFAULT 'pending',
    task_context JSONB,
    trace_id VARCHAR(100),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Create acknowledgements table
CREATE TABLE IF NOT EXISTS acknowledgements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id),
    agent_id UUID NOT NULL REFERENCES agents(id),
    status VARCHAR(50) NOT NULL,
    details TEXT,
    processed_at TIMESTAMP WITH TIME ZONE,
    nonce VARCHAR(100) UNIQUE NOT NULL,
    signature TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    type VARCHAR(50) NOT NULL,
    filter JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING GIN(capabilities);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_messages_trace_id ON messages(trace_id);

CREATE INDEX IF NOT EXISTS idx_acks_message_id ON acknowledgements(message_id);
CREATE INDEX IF NOT EXISTS idx_acks_nonce ON acknowledgements(nonce);

CREATE INDEX IF NOT EXISTS idx_subs_agent_id ON subscriptions(agent_id);
CREATE INDEX IF NOT EXISTS idx_subs_tenant_id ON subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_subs_status ON subscriptions(status);
