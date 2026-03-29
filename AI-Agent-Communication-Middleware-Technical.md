# AI Agents 通信中间件 - 技术架构详解

> 版本: v1.0
> 日期: 2025-03-28
> 状态: 设计阶段

---

## 目录

1. [系统整体架构](#1-系统整体架构)
2. [技术栈选型](#2-技术栈选型)
3. [数据模型设计](#3-数据模型设计)
4. [核心服务设计](#4-核心服务设计)
5. [API 设计](#5-api-设计)
6. [SDK 设计](#6-sdk-设计)
7. [部署架构](#7-部署架构)
8. [监控与运维](#8-监控与运维)

---

## 1. 系统整体架构

### 1.1 架构总览

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           AgentMessaging Platform                              │
│                         (企业级 Agent 通信中间件)                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                           客户端层 (Clients)                              │ │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │ │
│  │  │ Python  │  │  Node   │  │   Go    │  │  Rust   │  │  HTTP   │    │ │
│  │  │   SDK   │  │    JS   │  │   SDK   │  │   SDK   │  │  REST   │    │ │
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘    │ │
│  └───────┼────────────┼────────────┼────────────┼────────────┼──────────┘ │
│          └────────────┴────────────┴────────────┴────────────┘            │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         边缘层 (Edge Layer)                              │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │ │
│  │  │                    Global Load Balancer                           │  │ │
│  │  │                   (AWS ALB / Cloudflare)                         │  │ │
│  │  └─────────────────────────────────────────────────────────────────┘  │ │
│  │                                    │                                    │ │
│  │         ┌──────────────────────────┼──────────────────────────┐        │ │
│  │         ▼                          ▼                          ▼        │ │
│  │  ┌─────────────┐          ┌─────────────┐          ┌─────────────┐    │ │
│  │  │  API GW #1  │          │  API GW #2  │          │  API GW #3  │    │ │
│  │  │ (us-east-1) │          │ (eu-west-1) │          │ (ap-south-1)│    │ │
│  │  └──────┬──────┘          └──────┬──────┘          └──────┬──────┘    │ │
│  └─────────┼──────────────────────────┼──────────────────────────┼──────────┘ │
│            └──────────────────────────┴──────────────────────────┘            │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      协议适配层 (Protocol Layer)                          │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │ │
│  │  │    MCP      │  │    A2A      │  │  WebSocket  │  │   REST      │   │ │
│  │  │   Adapter   │  │   Adapter   │  │   Handler   │  │   Handler   │   │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      核心服务层 (Core Services)                           │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐    │ │
│  │  │                      消息路由层 (Message Router)                 │    │ │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐          │    │ │
│  │  │  │  Direct │  │  Match  │  │  Fanout │  │  Topic  │          │    │ │
│  │  │  │  Route  │  │  Route  │  │  Route  │  │  Route  │          │    │ │
│  │  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘          │    │ │
│  │  └─────────────────────────────────────────────────────────────────┘    │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │
│  │  │   Session   │  │  Capability │  │    Task     │  │   Message   │ │ │
│  │  │   Manager   │  │   Registry  │  │   Tracker   │  │   Broker    │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘ │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      可靠性层 (Reliability Layer)                         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │ │
│  │  │   Dead      │  │   Retry    │  │    Rate     │  │   Circuit   │   │ │
│  │  │   Letter    │  │   Engine   │  │   Limit     │  │   Breaker   │   │ │
│  │  │   Queue     │  │            │  │             │  │             │   │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         存储层 (Storage Layer)                           │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │
│  │  │ PostgreSQL  │  │   Redis     │  │    S3       │  │   Kafka     │ │ │
│  │  │  (主数据)   │  │  (缓存/队列) │  │  (大消息)  │  │  (日志)    │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘ │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 架构分层说明

| 层级 | 职责 | 关键组件 |
|------|------|----------|
| **客户端层** | 多语言 SDK | Python, Node.js, Go, Rust SDK |
| **边缘层** | 全球负载均衡、SSL 终止 | AWS ALB, Cloudflare |
| **协议适配层** | 协议转换 | MCP Adapter, A2A Adapter, WebSocket Handler |
| **核心服务层** | 业务逻辑 | Router, Registry, Tracker, Broker |
| **可靠性层** | 容错保障 | DLQ, Retry, Rate Limiter, Circuit Breaker |
| **存储层** | 数据持久化 | PostgreSQL, Redis, S3, Kafka |

---

## 2. 技术栈选型

### 2.1 核心技术栈

| 组件 | 选型 | 原因 |
|------|------|------|
| **核心运行时** | Go 1.21+ | 高并发、低延迟、成熟的分布式系统生态 |
| **SDK 语言** | Python, Node.js, Go, Rust | 覆盖主流 AI 开发语言 |
| **Web 框架** | Gin (Go) | 性能好、学习曲线低 |
| **WebSocket** | gorilla/websocket | Go 生态最成熟 |
| **数据库** | PostgreSQL 15 | 可靠、支持 JSONB、成熟 |
| **缓存/队列** | Redis 7 Cluster | Redis Streams 实现消息队列 |
| **消息日志** | Kafka 3.x | 日志聚合、事件溯源 |
| **服务网格** | Envoy | mTLS、流量管理、可观测性 |
| **容器编排** | Kubernetes (EKS/GKE) | 成熟的云原生方案 |
| **监控** | Prometheus + Grafana | CNCF 生态、成熟方案 |
| **日志** | Loki | 与 Grafana 集成好 |
| **追踪** | Jaeger | 分布式追踪标准 |
| **CI/CD** | GitHub Actions + ArgoCD | GitOps 部署 |

### 2.2 技术选型理由

**为什么选择 Go 而不是 Java/Rust?**

| 语言 | 并发模型 | 学习曲线 | 生态 | 适用场景 |
|------|----------|----------|------|----------|
| Go | goroutine | 低 | 丰富 | 分布式服务 ✅ |
| Java | thread | 高 | 丰富但重 | 企业级应用 |
| Rust | async/await | 高 | 一般 | 系统编程 |

---

## 3. 数据模型设计

### 3.1 核心实体关系

```
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│   Tenant    │       │    Agent    │       │   Message   │
├─────────────┤       ├─────────────┤       ├─────────────┤
│ id          │──┐    │ id          │       │ id          │
│ name        │  │    │ tenant_id   │◀──┐   │ sender_id   │◀──┐
│ plan        │  │    │ did         │   │   │ tenant_id   │   │
│ limits      │  │    │ public_key  │   │   │ content     │   │
│ usage       │  │    │ capabilities│   │   │ metadata    │   │
└─────────────┘  │    │ status      │   │   │ status      │   │
                 │    └─────────────┘   │   └─────────────┘   │
                 │           │           │          │           │
                 │           │           │          │           │
                 │    ┌───────┴───────┐   │   ┌─────┴─────┐    │
                 │    │ Subscription  │   │   │     Ack   │    │
                 │    ├───────────────┤   │   ├───────────┤    │
                 │    │ id            │   │   │ id        │    │
                 │    │ agent_id     │◀──┘   │ message_id│◀───┘
                 └───▶│ tenant_id    │       │ agent_id  │
                      │ type         │       │ status    │
                      │ filter       │       │ nonce     │
                      └─────────────┘       └───────────┘
```

### 3.2 数据库 Schema

```sql
-- Tenant 表
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan VARCHAR(50) DEFAULT 'standard',
    limits JSONB NOT NULL,
    usage JSONB NOT NULL,
    message_used BIGINT DEFAULT 0,
    agent_count INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    sso_enabled BOOLEAN DEFAULT false,
    billing_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Agent 表
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    did VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    name VARCHAR(255),
    version VARCHAR(50),
    provider VARCHAR(100),
    tier VARCHAR(50) DEFAULT 'free',
    capabilities JSONB NOT NULL,
    endpoints JSONB,
    trust_level INT DEFAULT 1,
    verified_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'offline',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT agents_tenant_status CHECK (status IN ('online', 'away', 'busy', 'offline'))
);

CREATE INDEX idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_capabilities ON agents USING GIN(capabilities);

-- Message 表
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_type VARCHAR(50) NOT NULL,
    sender_id UUID NOT NULL REFERENCES agents(id),
    recipient_ids TEXT NOT NULL, -- JSON array of UUIDs
    content BYTEA NOT NULL,
    content_size INT NOT NULL,
    content_type VARCHAR(100),
    metadata JSONB,
    delivery_guarantee VARCHAR(50) DEFAULT 'at_least_once',
    status VARCHAR(50) DEFAULT 'pending',
    task_context JSONB,
    trace_id VARCHAR(100),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    processed_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT messages_status CHECK (status IN ('pending', 'sent', 'delivered', 'processed', 'failed', 'dead_letter'))
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_sender_id ON messages(sender_id);
CREATE INDEX idx_messages_status ON messages(status);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_messages_tenant_id ON messages(tenant_id);
CREATE INDEX idx_messages_trace_id ON messages(trace_id);

-- Acknowledgement 表
CREATE TABLE acknowledgements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id),
    agent_id UUID NOT NULL REFERENCES agents(id),
    status VARCHAR(50) NOT NULL,
    details TEXT,
    processed_at TIMESTAMP WITH TIME ZONE,
    nonce VARCHAR(100) UNIQUE NOT NULL,
    signature TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT ack_status CHECK (status IN ('received', 'processed', 'rejected', 'failed'))
);

CREATE INDEX idx_acks_message_id ON acknowledgements(message_id);
CREATE INDEX idx_acks_nonce ON acknowledgements(nonce);

-- Subscription 表
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    type VARCHAR(50) NOT NULL,
    filter JSONB NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT sub_type CHECK (type IN ('direct', 'capability', 'topic', 'pattern'))
);

CREATE INDEX idx_subs_agent_id ON subscriptions(agent_id);
CREATE INDEX idx_subs_tenant_id ON subscriptions(tenant_id);
CREATE INDEX idx_subs_status ON subscriptions(status);
```

---

## 4. 核心服务设计

### 4.1 MessageEngine

```go
// engine/message_engine.go

package engine

type MessageEngine struct {
    redis       *redis.Client
    pgDB       *storage.PostgresDB
    router     *MessageRouter
    ackEngine  *AckEngine
    dlq        *DeadLetterQueue
    config     *EngineConfig
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
    mu         sync.RWMutex
    running    bool
}

type EngineConfig struct {
    WorkerCount      int
    BatchSize        int
    FlushInterval    time.Duration
    MaxRetries       int
    RetryBaseDelay   time.Duration
}

// SendMessage 发送消息
func (e *MessageEngine) SendMessage(ctx context.Context, msg *model.Message) (*SendResult, error) {
    // 1. 验证发送者
    agent, err := e.pgDB.GetAgent(ctx, msg.SenderID)
    if err != nil {
        return nil, fmt.Errorf("invalid sender: %w", err)
    }

    // 2. 解析接收者
    recipients, err := e.resolveRecipients(ctx, msg)
    if err != nil {
        return nil, fmt.Errorf("resolve recipients: %w", err)
    }

    // 3. 持久化消息
    if err := e.pgDB.CreateMessage(ctx, msg); err != nil {
        return nil, fmt.Errorf("persist message: %w", err)
    }

    // 4. 路由消息
    routes, err := e.router.Route(ctx, msg, recipients)
    if err != nil {
        return nil, fmt.Errorf("route message: %w", err)
    }

    result := &SendResult{
        MessageID:   msg.ID,
        Routes:      routes,
        DeliveredAt: time.Now(),
    }

    // 5. 根据 delivery guarantee 处理
    switch msg.DeliveryGuarantee {
    case model.DeliveryAtMostOnce:
        e.dispatchAsync(ctx, msg, routes)
    case model.DeliveryAtLeastOnce:
        e.dispatchWithAck(ctx, msg, routes)
    case model.DeliveryExactlyOnce:
        e.dispatchExactlyOnce(ctx, msg, routes)
    }

    return result, nil
}
```

### 4.2 MessageRouter

```go
// engine/router.go

type MessageRouter struct {
    strategies map[RouterStrategy]RoutingStrategy
}

type RoutingStrategy interface {
    Route(ctx context.Context, msg *model.Message, recipients []uuid.UUID) ([]Route, error)
}

// DirectRouting 直接路由
type DirectRouting struct{}

func (r *DirectRouting) Route(ctx context.Context, msg *model.Message, recipients []uuid.UUID) ([]Route, error) {
    routes := make([]Route, 0, len(recipients))
    for _, recipientID := range recipients {
        routes = append(routes, Route{
            RecipientID: recipientID,
            Channel:     fmt.Sprintf("agent:%s:queue", recipientID),
            Priority:    0,
        })
    }
    return routes, nil
}

// CapabilityRouting 能力路由
type CapabilityRouting struct{}

func (r *CapabilityRouting) Route(ctx context.Context, msg *model.Message, recipients []uuid.UUID) ([]Route, error) {
    if msg.TaskContext == nil {
        return nil, fmt.Errorf("task context required for capability routing")
    }

    matchedAgents, err := e.pgDB.FindAgentsByCapability(ctx, &model.CapabilityQuery{
        Types: []model.CapabilityType{model.CapabilityType(msg.Metadata.Custom["requiredCapability"])},
        Status: model.AgentStatusOnline,
    })

    routes := make([]Route, 0, len(matchedAgents))
    for _, agent := range matchedAgents {
        routes = append(routes, Route{
            RecipientID: agent.ID,
            Channel:     fmt.Sprintf("agent:%s:queue", agent.ID),
            Priority:    msg.TaskContext.Priority,
        })
    }
    return routes, nil
}
```

### 4.3 AckEngine

```go
// engine/ack_engine.go

type AckEngine struct {
    redis           *redis.Client
    pgDB            *storage.PostgresDB
    idempotencyCache *redis.Cache
    pendingAcks     map[uuid.UUID]*PendingAck
    mu              sync.RWMutex
    ackTimeout      time.Duration
    maxRetries      int
}

type PendingAck struct {
    MessageID   uuid.UUID
    Recipients  []uuid.UUID
    Status      map[uuid.UUID]model.AckStatus
    CreatedAt   time.Time
    LastRetry   time.Time
    RetryCount  int
}

func (e *AckEngine) HandleAck(ctx context.Context, ack *model.Acknowledgement) error {
    // 1. 验证签名
    if err := e.verifyAckSignature(ack); err != nil {
        return err
    }

    // 2. 检查幂等性
    cacheKey := e.generateIdempotencyKey(ack.MessageID, ack.AgentID)
    if e.idempotencyCache.Exists(ctx, cacheKey) {
        return nil
    }

    // 3. 更新状态
    e.mu.Lock()
    pending, ok := e.pendingAcks[ack.MessageID]
    pending.Status[ack.AgentID] = ack.Status
    e.mu.Unlock()

    // 4. 持久化确认
    if err := e.pgDB.CreateAck(ctx, ack); err != nil {
        return err
    }

    // 5. 标记幂等性
    e.idempotencyCache.Set(ctx, cacheKey, "1", 24*time.Hour)

    // 6. 检查是否所有接收者都已确认
    e.checkAllAcksReceived(ack.MessageID)
    return nil
}
```

### 4.4 DeadLetterQueue

```go
// engine/dlq.go

type DeadLetterQueue struct {
    redis          *redis.Client
    pgDB           *storage.PostgresDB
    maxRetries     int
    retryDelays    []time.Duration
    deadLetterTTL  time.Duration
}

type DLQEntry struct {
    ID         uuid.UUID
    MessageID  uuid.UUID
    Message    *model.Message
    Reason     DLQReason
    RetryCount int
    EnqueuedAt time.Time
    TTL        time.Duration
}

func (q *DeadLetterQueue) Enqueue(ctx context.Context, msg *model.Message, reason DLQReason) error {
    dlqEntry := &DLQEntry{
        ID:        uuid.New(),
        MessageID: msg.ID,
        Message:   msg,
        Reason:    reason,
        RetryCount: msg.TaskContext.RetryCount,
        EnqueuedAt: time.Now(),
        TTL:        q.deadLetterTTL,
    }

    key := fmt.Sprintf("dlq:%s", msg.ID)
    data, _ := json.Marshal(dlqEntry)

    if err := q.redis.ZAdd(ctx, "dlq:pending", redis.Z{
        Score:  float64(time.Now().Unix()),
        Member: data,
    }).Err(); err != nil {
        return err
    }

    return q.pgDB.UpdateMessageStatus(ctx, msg.ID, model.MessageStatusDeadLetter)
}
```

---

## 5. API 设计

### 5.1 REST API

```go
// api/rest/server.go

func (s *Server) setupRoutes() {
    v1 := s.router.Group("/api/v1")

    // Auth middleware
    v1.Use(s.authMiddleware.Authenticate())
    v1.Use(middleware.TenantContext())

    // Agent routes
    agents := v1.Group("/agents")
    {
        agents.POST("", s.agentHandler.Register)
        agents.GET("", s.agentHandler.List)
        agents.GET("/:id", s.agentHandler.Get)
        agents.PUT("/:id", s.agentHandler.Update)
        agents.DELETE("/:id", s.agentHandler.Deregister)
        agents.POST("/:id/heartbeat", s.agentHandler.Heartbeat)
    }

    // Message routes
    messages := v1.Group("/messages")
    {
        messages.POST("", s.messageHandler.Send)
        messages.POST("/batch", s.messageHandler.SendBatch)
        messages.GET("", s.messageHandler.List)
        messages.GET("/:id", s.messageHandler.Get)
        messages.POST("/:id/ack", s.messageHandler.Acknowledge)
        messages.GET("/:id/status", s.messageHandler.GetStatus)
    }

    // Subscription routes
    subs := v1.Group("/subscriptions")
    {
        subs.POST("", s.subHandler.Create)
        subs.GET("", s.subHandler.List)
        subs.DELETE("/:id", s.subHandler.Delete)
    }

    // Discovery routes
    discovery := v1.Group("/discovery")
    {
        discovery.POST("/query", s.agentHandler.QueryByCapability)
    }
}
```

### 5.2 API 请求/响应示例

```bash
# 发送消息
POST /api/v1/messages
Content-Type: application/json
Authorization: Bearer <token>

{
  "messageType": "task.request",
  "recipients": ["550e8400-e29b-41d4-a716-446655440000"],
  "content": {
    "description": "帮我写一个排序算法",
    "language": "python"
  },
  "deliveryGuarantee": "at_least_once",
  "metadata": {
    "tags": {"priority": "high"},
    "correlationId": "req-123"
  },
  "taskContext": {
    "priority": 2,
    "deadline": 1735689600000
  }
}

# 响应
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "sent",
  "deliveredAt": null
}
```

### 5.3 WebSocket 协议

```typescript
// WebSocket 连接
// wss://api.agentmsg.cloud/api/v1/ws?token=xxx&agent_id=yyy

// 发送消息
{
  "type": "message",
  "id": "msg-uuid",
  "conversationId": "conv-uuid",
  "subType": "task.request",
  "recipients": ["agent-uuid-1", "agent-uuid-2"],
  "content": {"task": "do something"},
  "metadata": {},
  "deliveryGuarantee": "at_least_once",
  "taskContext": {
    "taskId": "task-uuid",
    "priority": 2
  }
}

// 接收消息
{
  "type": "message",
  "id": "msg-uuid",
  "conversationId": "conv-uuid",
  "subType": "task.request",
  "sender": {
    "agentId": "agent-uuid",
    "name": "My Agent",
    "capabilities": [...]
  },
  "content": {"task": "do something"},
  "metadata": {},
  "createdAt": 1735689600000
}

// 确认
{
  "type": "ack",
  "id": "ack-uuid",
  "messageId": "msg-uuid",
  "status": "processed",
  "processedAt": 1735689600100
}
```

---

## 6. SDK 设计

### 6.1 Python SDK

```python
# agentmsg/client.py

class AgentMsgClient:
    def __init__(
        self,
        api_key: str,
        agent_id: str,
        endpoint: str = "wss://api.agentmsg.cloud",
        reconnect: bool = True,
        max_retries: int = 3,
    ):
        self.api_key = api_key
        self.agent_id = agent_id
        self.endpoint = endpoint
        self.reconnect = reconnect
        self.max_retries = max_retries
        self._ws = None
        self._running = False
        self._message_handlers = []

    async def connect(self) -> None:
        """Connect to AgentMsg cloud."""
        self._ws_url = f"{self.endpoint}/api/v1/ws?token={self.api_key}&agent_id={self.agent_id}"
        self._ws = await websockets.connect(self._ws_url)
        self._running = True
        asyncio.create_task(self._message_loop())

    async def send_message(
        self,
        content: Any,
        recipients: List[str],
        message_type: MessageType = MessageType.GENERIC,
        delivery: DeliveryGuarantee = DeliveryGuarantee.AT_LEAST_ONCE,
    ) -> SendResult:
        """Send a message to agents."""
        message = Message(
            id=str(uuid.uuid4()),
            conversation_id=str(uuid.uuid4()),
            message_type=message_type,
            sender_id=self.agent_id,
            recipients=recipients,
            content=content,
            delivery_guarantee=delivery,
        )

        ws_message = message.to_ws_dict()
        await self._ws.send(json.dumps(ws_message))

        return SendResult(message_id=message.id, status="sent")

    def on_message(self, handler: Callable[[Message], None]) -> None:
        """Register a message handler."""
        self._message_handlers.append(handler)

    async def receive_messages(self) -> AsyncIterator[Message]:
        """Async iterator for incoming messages."""
        queue = asyncio.Queue()
        async def handler(data): await queue.put(Message.from_dict(data))
        self.on_message(handler)
        while self._running:
            try:
                yield await asyncio.wait_for(queue.get(), timeout=1.0)
            except asyncio.TimeoutError:
                continue
```

### 6.2 Go SDK

```go
// go-sdk/agentmsg/client.go

package agentmsg

type Client struct {
    APIKey   string
    AgentID  uuid.UUID
    Endpoint string

    ws      *websocket.Conn
    httpURL string
    wsURL   string

    reconnect   bool
    maxRetries int

    messageHandlers []MessageHandler
    ackHandlers    map[uuid.UUID]chan *models.Acknowledgement

    ctx    context.Context
    cancel context.CancelFunc
}

func NewClient(apiKey string, agentID uuid.UUID, opts ...Option) (*Client, error) {
    cfg := &Config{
        Endpoint:  "https://api.agentmsg.cloud",
        Reconnect: true,
    }
    for _, opt := range opts {
        opt(cfg)
    }

    c := &Client{
        APIKey:   apiKey,
        AgentID:  agentID,
        httpURL:  cfg.Endpoint,
        wsURL:    fmt.Sprintf("wss://%s/api/v1/ws?token=%s&agent_id=%s", cfg.Endpoint, apiKey, agentID),
        messageHandlers: make([]MessageHandler, 0),
        ackHandlers:     make(map[uuid.UUID]chan *models.Acknowledgement),
        ctx: context.WithCancel(context.Background()),
    }
    return c, nil
}

func (c *Client) Connect(ctx context.Context) error {
    ws, _, err := websocket.DefaultDialer.DialContext(ctx, c.wsURL, nil)
    if err != nil {
        return fmt.Errorf("websocket dial: %w", err)
    }
    c.ws = ws
    go c.readLoop()
    go c.heartbeatLoop()
    return nil
}

func (c *Client) SendMessage(ctx context.Context, msg *models.Message) (*models.SendResult, error) {
    data, err := json.Marshal(msg)
    if err != nil {
        return nil, err
    }

    wsMsg := WSMessage{Type: "message", ID: msg.ID.String(), Content: data}
    if err := c.ws.WriteJSON(wsMsg); err != nil {
        return nil, err
    }

    return &models.SendResult{MessageID: msg.ID, Status: "sent"}, nil
}
```

---

## 7. 部署架构

### 7.1 Kubernetes 部署

```yaml
# k8s/api-gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
  namespace: agentmsg
spec:
  replicas: 6
  selector:
    matchLabels:
      app: api-gateway
  template:
    metadata:
      labels:
        app: api-gateway
    spec:
      containers:
        - name: api-gateway
          image: registry.agentmsg.cloud/api-gateway:v1.0.0
          ports:
            - name: http
              containerPort: 8080
            - name: ws
              containerPort: 8081
          env:
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: agentmsg-secrets
                  key: redis-url
          resources:
            requests:
              cpu: 1000m
              memory: 1Gi
            limits:
              cpu: 2000m
              memory: 2Gi
```

### 7.2 多区域部署

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              全球部署架构                                          │
├─────────────────────────────────────────────────────────────────────────────────┤
│
│                           Global Traffic Manager                                 │
│                        (AWS Route53 / Cloudflare)                                │
│                                    │                                              │
│          ┌─────────────────────────┼─────────────────────────┐                  │
│          │                         │                         │                  │
│          ▼                         ▼                         ▼                  │
│   ┌─────────────┐          ┌─────────────┐          ┌─────────────┐           │
│   │  us-east-1  │          │  eu-west-1  │          │ ap-south-1  │           │
│   │   (美东)    │          │   (西欧)    │          │  (孟买)     │           │
│   └──────┬──────┘          └──────┬──────┘          └──────┬──────┘           │
│          │                        │                        │                  │
│   ┌──────┴──────┐          ┌──────┴──────┐          ┌──────┴──────┐           │
│   │   API GW    │          │   API GW    │          │   API GW    │           │
│   │   x 6      │          │   x 4       │          │   x 4       │           │
│   └─────────────┘          └─────────────┘          └─────────────┘           │
│
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 8. 监控与运维

### 8.1 监控指标

| 指标类别 | 指标名称 | 描述 | 告警阈值 |
|----------|----------|------|----------|
| **消息引擎** | engine_messages_total | 消息总数 | - |
| | engine_message_duration_p99 | P99 延迟 | > 50ms |
| | engine_queue_depth | 队列深度 | > 10000 |
| **API 网关** | api_requests_total | 请求总数 | - |
| | api_request_duration_p99 | P99 延迟 | > 200ms |
| | api_error_rate | 错误率 | > 1% |
| **WebSocket** | ws_connections_active | 活跃连接 | > 80000 |
| | ws_messages_per_second | 消息速率 | - |
| **系统** | cpu_usage | CPU 使用率 | > 80% |
| | memory_usage | 内存使用率 | > 85% |

### 8.2 告警规则

```yaml
# prometheus-rules.yaml
groups:
  - name: agentmsg
    rules:
      - alert: MessageEngineDown
        expr: up{job="message-engine"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Message Engine 实例不可用"

      - alert: MessageEngineHighLatency
        expr: histogram_quantile(0.99, rate(engine_message_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "消息处理延迟过高"

      - alert: APIGatewayHighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API 错误率过高"

      - alert: WebSocketConnectionsHigh
        expr: ws_connections_active > 80000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "WebSocket 连接数接近上限"
```

---

## 附录

### A. 性能指标目标

| 指标 | 目标值 |
|------|--------|
| 吞吐量 | 100K+ 消息/秒 |
| P99 延迟 | < 50ms |
| WebSocket 并发 | 100K+ 连接/节点 |
| SLA | 99.99% |
| 可用性 | 99.99% |

### B. 项目代码量估算

| 模块 | 代码量 | 说明 |
|------|--------|------|
| 核心消息引擎 | ~8,000 行 | 路由、持久化、确认 |
| API Gateway | ~5,000 行 | HTTP/WebSocket/认证 |
| 协议适配器 | ~3,000 行 | MCP/A2A |
| Python SDK | ~2,000 行 | 完整 SDK |
| Node.js SDK | ~2,000 行 | 完整 SDK |
| Go SDK | ~1,500 行 | 完整 SDK |
| 运维脚本 | ~2,000 行 | K8s/Helm |
| **总计** | **~25,000 行** | |

---

*本文档为技术架构设计文档，具体实现需根据项目进展调整。*
