# AgentMsg - AI Agent Communication Middleware

Enterprise-grade messaging infrastructure for AI agents.

## Overview

AgentMsg is a messaging platform designed specifically for AI Agent communication. It provides reliable message delivery, capability discovery, and task coordination between agents.

## Features

- **Reliable Message Delivery**: At-most-once, At-least-once, and Exactly-once delivery guarantees
- **Capability Discovery**: Find agents by their capabilities
- **Task Coordination**: Send task requests and receive responses
- **Multi-language SDKs**: Python, Node.js, Go, and more
- **Operational Readiness**: Readiness probes, Prometheus metrics, audit logs, rate limiting, DLQ retries, and OpenTelemetry tracing

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Redis 7+
- Docker (optional)

### Installation

```bash
# Clone the repository
git clone https://github.com/haocn-ops/agentmsg.git
cd agentmsg

# Install dependencies
make deps

# Run embedded database migrations
make migrate

# Start the services
make run

# Verify health, readiness, and metrics endpoints
make smoke
```

### Using Docker

```bash
# Start all services
docker compose -f deployments/docker/docker-compose.yml up -d

# View logs
docker compose -f deployments/docker/docker-compose.yml logs -f
```

## SDKs

### Python

```bash
pip install agentmsg
```

```python
from agentmsg import Client, MessageType, DeliveryGuarantee

async with Client(api_key="your-api-key", agent_id="your-agent-id") as client:
    result = await client.send_message(
        content={"text": "Hello!"},
        recipients=["recipient-id"],
        message_type=MessageType.GENERIC
    )
```

### Node.js

```bash
npm install agentmsg
```

```typescript
import { AgentMsgClient, MessageType } from 'agentmsg';

const client = new AgentMsgClient({
  apiKey: 'your-api-key',
  agentId: 'your-agent-id'
});

await client.connect();
await client.sendMessage(
  { text: 'Hello!' },
  ['recipient-id'],
  MessageType.GENERIC
);
```

SDK notes:

- Current SDKs use the implemented REST API for send, heartbeat, discovery, and subscriptions.
- Realtime WebSocket hooks are still experimental client-side code because the current server build does not expose `/api/v1/ws`.
- Capability updates assume the authenticated agent already exists on the server.

### Go

```bash
go get agentmsg/sdk/go
```

```go
config := &agentmsg.ClientConfig{
    APIKey:   "your-api-key",
    AgentID:  agentID,
    TenantID: tenantID,
}

client, _ := agentmsg.NewClient(config)
ctx := context.Background()
client.Connect(ctx)

result, _ := client.SendMessage(ctx, &agentmsg.Message{
    ConversationID: uuid.New(),
    MessageType:   agentmsg.MessageTypeGeneric,
    Content:       []byte("Hello!"),
})
```

## API Reference

### REST API

Base URL: `https://api.agentmsg.cloud/api/v1`

#### Authentication

```bash
curl -H "Authorization: Bearer YOUR_JWT" \
     https://api.agentmsg.cloud/api/v1/agents
```

Tenant isolation:

- Resource reads and writes are scoped to the authenticated tenant.
- Cross-tenant access returns `404` to avoid leaking resource existence.
- Message acknowledgements are only accepted from recipient agents of that message.

#### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/agents` | Register a new agent |
| GET | `/agents` | List all agents |
| GET | `/agents/:id` | Get agent by ID |
| POST | `/messages` | Send a message |
| POST | `/messages/batch` | Send batch messages |
| GET | `/messages/:id` | Get message by ID |
| POST | `/subscriptions` | Create subscription |
| POST | `/discovery/query` | Query capabilities |

Error response shape:

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

### WebSocket

WebSocket transport is not exposed by the current server build yet.

The SDK folders still contain experimental client-side WebSocket code, but `/api/v1/ws` is not implemented in the API gateway at this stage.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Clients                              │
│    ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│    │ Python   │  │ Node.js  │  │   Go     │               │
│    └────┬─────┘  └────┬─────┘  └────┬─────┘               │
└──────────┼────────────┼────────────┼────────────────────────┘
           │            │            │
           └────────────┼────────────┘
                        │ REST/WebSocket
           ┌────────────┴────────────┐
           │      API Gateway         │
           │   (Load Balancing)      │
           └────────────┬────────────┘
                        │
           ┌────────────┴────────────┐
           │     Message Engine      │
           │   ┌─────────────────┐   │
           │   │  Ack Engine     │   │
           │   │  DLQ            │   │
           │   │  Router         │   │
           │   └─────────────────┘   │
           └────────────┬────────────┘
                        │
      ┌─────────────────┼─────────────────┐
      │                 │                  │
      ▼                 ▼                  ▼
┌──────────┐     ┌──────────┐     ┌──────────┐
│PostgreSQL│     │  Redis   │     │ Workers  │
│(Storage) │     │ (Queue)  │     │          │
└──────────┘     └──────────┘     └──────────┘
```

## Deployment

### Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/

# Check status
kubectl get pods -n agentmsg
```

Operational rollout notes live in [`docs/operations.md`](docs/operations.md).

### Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `JWT_SECRET` | HMAC secret for agent JWTs | `dev-secret` |
| `AUTO_MIGRATE` | Run embedded SQL migrations on startup | `false` |
| `OTEL_ENABLED` | Enable OpenTelemetry tracing export | `false` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP collector endpoint, required when tracing is enabled | - |
| `OTEL_INSECURE` | Disable TLS for OTLP exporter | `true` |
| `RATE_LIMIT_REQUESTS` | Redis-backed request budget per window | `600` |
| `RATE_LIMIT_WINDOW_SECONDS` | Rate limit window size in seconds | `60` |
| `LOG_LEVEL` | Logging level | `info` |
| `ENV` | Deployment environment label | `development` |

Production guardrails:

- API Gateway refuses to start in `production` with the default development `JWT_SECRET`.
- Any service refuses to start with `OTEL_ENABLED=true` and no `OTEL_EXPORTER_OTLP_ENDPOINT`.

Operational endpoints:

| Service | Endpoint | Purpose |
|---------|----------|---------|
| API Gateway | `:8080/health` | Liveness probe |
| API Gateway | `:8080/ready` | PostgreSQL and Redis readiness |
| API Gateway | `:8080/metrics` | In-band Prometheus metrics |
| API Gateway | `:9090/metrics` | Dedicated metrics server |
| Message Engine | `:8081/health` | Liveness probe |
| Message Engine | `:8081/ready` | PostgreSQL and Redis readiness |
| Message Engine | `:9091/metrics` | Dedicated metrics server |

## Development

```bash
# Run tests
make test

# Run startup smoke checks
make smoke

# Build binaries
make build

# Run linter
make lint
```

## License

MIT License - see [LICENSE](LICENSE) for details.
