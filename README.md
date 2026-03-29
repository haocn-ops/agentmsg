# AgentMsg - AI Agent Communication Middleware

Enterprise-grade messaging infrastructure for AI agents.

## Overview

AgentMsg is a messaging platform designed specifically for AI Agent communication. It provides reliable message delivery, capability discovery, and task coordination between agents.

## Features

- **Reliable Message Delivery**: At-most-once, At-least-once, and Exactly-once delivery guarantees
- **Capability Discovery**: Find agents by their capabilities
- **Task Coordination**: Send task requests and receive responses
- **Real-time Communication**: WebSocket-based push messaging
- **Multi-language SDKs**: Python, Node.js, Go, and more

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Redis 7+
- Docker (optional)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/agentmsg.git
cd agentmsg

# Install dependencies
make deps

# Run database migrations
make migrate

# Start the services
make run
```

### Using Docker

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

## SDKs

### Python

```bash
pip install agentmsg
```

```python
from agentmsg import Client, MessageType, DeliveryGuarantee

async with Client(api_key="your-api-key", agent_id="your-agent-id") as client:
    await client.register_capabilities([...])
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
import { AgentMsgClient } from 'agentmsg';

const client = new AgentMsgClient({
  apiKey: 'your-api-key',
  agentId: 'your-agent-id'
});

await client.connect();
await client.sendMessage({
  content: { text: 'Hello!' },
  recipients: ['recipient-id'],
  messageType: 'generic'
});
```

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
curl -H "Authorization: Bearer YOUR_API_KEY" \
     -H "X-Agent-ID: YOUR_AGENT_ID" \
     https://api.agentmsg.cloud/api/v1/agents
```

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

### WebSocket

Connect to: `wss://ws.agentmsg.cloud/ws`

```javascript
const ws = new WebSocket('wss://ws.agentmsg.cloud/ws?token=YOUR_TOKEN&agent_id=YOUR_AGENT_ID');

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Received:', msg);
};
```

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

### Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `API_KEY` | API authentication key | - |
| `LOG_LEVEL` | Logging level | `info` |

## Development

```bash
# Run tests
make test

# Run with hot reload
make dev

# Build binaries
make build

# Run linter
make lint
```

## License

MIT License - see [LICENSE](LICENSE) for details.
