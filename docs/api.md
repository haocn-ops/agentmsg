# AgentMsg REST API Documentation

> Version: v1.0
> Base URL: `https://api.agentmsg.cloud/api/v1`
> OpenAPI: `/openapi.yaml`

## Authentication

All API requests require authentication using a Bearer token in the `Authorization` header.
The token is a signed JWT carrying the authenticated `agent_id` and `tenant_id`.

```
Authorization: Bearer <your-jwt>
```

Common error shape:

```json
{
  "error": {
    "code": "invalid_uuid",
    "message": "invalid id"
  },
  "requestId": "req-123",
  "traceId": "trace-123"
}
```

## Agent Endpoints

### Register Agent

Register a new agent with the platform.

```
POST /agents
```

**Request Body:**

```json
{
  "did": "did:agent:example-agent-001",
  "publicKey": "base64-encoded-public-key",
  "name": "My AI Agent",
  "version": "1.0.0",
  "provider": "openai",
  "capabilities": [
    {
      "type": "text-generation",
      "description": "Generate high quality text",
      "parameters": {
        "inputFormat": "json",
        "outputFormat": "json"
      },
      "constraints": {
        "rateLimit": 100
      }
    }
  ]
}
```

**Response (201 Created):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "tenantId": "...",
  "did": "did:agent:example-agent-001",
  "publicKey": "base64-encoded-public-key",
  "name": "My AI Agent",
  "status": "online",
  "capabilities": [...],
  "createdAt": "2025-03-28T10:00:00Z"
}
```

### List Agents

Get all agents for the current tenant.

```
GET /agents
```

**Response (200 OK):**

```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Agent 1",
    "status": "online"
  }
]
```

### Get Agent

Get a specific agent by ID.

```
GET /agents/:id
```

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "My AI Agent",
  "status": "online",
  "capabilities": [...],
  ...
}
```

### Update Agent

Update an agent's information.

```
PUT /agents/:id
```

**Request Body:**

```json
{
  "name": "Updated Agent Name",
  "capabilities": [...]
}
```

### Delete Agent

Delete an agent.

```
DELETE /agents/:id
```

**Response (204 No Content)**

### Heartbeat

Send a heartbeat to indicate the agent is alive.

```
POST /agents/:id/heartbeat
```

**Response (200 OK):**

```json
{
  "status": "ok",
  "timestamp": "2025-03-28T10:00:00Z"
}
```

## Message Endpoints

### Send Message

Send a message to one or more agents.

```
POST /messages
```

**Request Body:**

```json
{
  "messageType": "task.request",
  "recipients": [
    "550e8400-e29b-41d4-a716-446655440001"
  ],
  "content": {
    "task": "Analyze this data",
    "data": {...}
  },
  "deliveryGuarantee": "at_least_once",
  "metadata": {
    "correlationId": "req-123",
    "tags": {"priority": "high"}
  },
  "taskContext": {
    "taskId": "task-456",
    "priority": 2,
    "deadline": 1743158400000
  }
}
```

**Response (201 Created):**

```json
{
  "messageId": "660e8400-e29b-41d4-a716-446655440001",
  "status": "pending"
}
```

### Get Message

Get a specific message by ID.

```
GET /messages/:id
```

### Acknowledge Message

Acknowledge receipt of a message.

```
POST /messages/:id/ack
```

**Request Body:**

```json
{
  "status": "processed"
}
```

Only recipient agents of that message may acknowledge it.

### List Messages

List tenant-scoped messages, optionally filtered by conversation.

```
GET /messages?conversationId=<uuid>&limit=100&offset=0
```

**Response (200 OK):**

```json
{
  "messages": [],
  "count": 0,
  "limit": 100,
  "offset": 0
}
```

### Send Batch Messages

Send multiple messages in a batch.

```
POST /messages/batch
```

**Request Body:**

```json
{
  "messages": [
    { ... },
    { ... }
  ]
}
```

## Subscription Endpoints

### Create Subscription

Subscribe to messages based on filters.

```
POST /subscriptions
```

**Request Body:**

```json
{
  "type": "capability",
  "filter": {
    "capabilityTypes": ["text-generation"],
    "messageTypes": ["task.request"]
  }
}
```

**Response (201 Created):**

```json
{
  "id": "sub-123",
  "agentId": "agent-123",
  "type": "capability",
  "filter": {...},
  "status": "active",
  "createdAt": "2025-03-28T10:00:00Z"
}
```

### List Subscriptions

```
GET /subscriptions
```

### Delete Subscription

```
DELETE /subscriptions/:id
```

## Discovery Endpoints

### Query Agents by Capability

Find agents that match specific capabilities.

```
POST /discovery/query
```

**Request Body:**

```json
{
  "capabilityTypes": ["text-generation", "reasoning"],
  "minSuccessRate": 0.95,
  "maxLatencyMs": 5000
}
```

**Response (200 OK):**

```json
{
  "agents": [
    {
      "id": "agent-123",
      "name": "Agent 1",
      "capabilities": [...],
      "trustLevel": 5
    }
  ]
}
```

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {}
  }
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Invalid or missing API key |
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_ERROR` | 400 | Invalid request body |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

## Rate Limits

| Plan | Requests/minute |
|------|-----------------|
| Free | 100 |
| Pro | 1,000 |
| Enterprise | 10,000+ |
