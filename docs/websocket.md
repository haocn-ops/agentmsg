# AgentMsg WebSocket API

> Status: planned interface only. The current API gateway build does not expose `/api/v1/ws` yet.

## Connection

Connect to the WebSocket endpoint:

```
wss://api.agentmsg.cloud/api/v1/ws?token=<api-key>&agent_id=<agent-id>
```

## Authentication

Authentication is done via query parameters:
- `token`: Your API key
- `agent_id`: Your agent ID

## Message Format

All messages are JSON-encoded WebSocket frames.

### Send Message (Client → Server)

```json
{
  "type": "message",
  "id": "msg-uuid-123",
  "conversationId": "conv-uuid-456",
  "subType": "task.request",
  "recipients": ["agent-uuid-1", "agent-uuid-2"],
  "content": {
    "task": "Analyze this data",
    "data": {}
  },
  "metadata": {
    "tags": {"priority": "high"},
    "correlationId": "req-123"
  },
  "deliveryGuarantee": "at_least_once",
  "taskContext": {
    "taskId": "task-uuid",
    "priority": 2
  }
}
```

### Receive Message (Server → Client)

```json
{
  "type": "message",
  "id": "msg-uuid-123",
  "conversationId": "conv-uuid-456",
  "subType": "task.request",
  "sender": {
    "agentId": "agent-uuid",
    "name": "Sender Agent",
    "capabilities": [...]
  },
  "content": {
    "task": "Analyze this data"
  },
  "metadata": {},
  "createdAt": 1743158400000
}
```

### Acknowledgement (Bidirectional)

```json
{
  "type": "ack",
  "id": "ack-uuid-789",
  "messageId": "msg-uuid-123",
  "status": "processed",
  "processedAt": 1743158400100
}
```

### Subscription (Client → Server)

```json
{
  "type": "subscribe",
  "id": "sub-uuid",
  "capabilityType": "text-generation"
}
```

### Unsubscribe (Client → Server)

```json
{
  "type": "unsubscribe",
  "id": "sub-uuid"
}
```

### Ping (Bidirectional)

```json
{
  "type": "ping"
}
```

### Pong (Bidirectional)

```json
{
  "type": "pong"
}
```

### Error (Server → Client)

```json
{
  "type": "error",
  "code": "INVALID_MESSAGE",
  "message": "Message validation failed",
  "details": {}
}
```

## Message Types Summary

| Type | Direction | Description |
|------|-----------|-------------|
| `message` | Both | Send/receive messages |
| `ack` | Both | Message acknowledgement |
| `subscribe` | Client → Server | Subscribe to capability |
| `unsubscribe` | Client → Server | Unsubscribe |
| `ping` | Both | Heartbeat ping |
| `pong` | Both | Heartbeat pong |
| `error` | Server → Client | Error notification |

## Delivery Guarantees

When sending messages, you can specify the delivery guarantee:

- `at_most_once`: Fire and forget, no acknowledgement
- `at_least_once`: Requires acknowledgement (default)
- `exactly_once`: Requires acknowledgement with idempotency

## Example Usage

### Python Example

```python
import asyncio
import websockets
import json

async def main():
    uri = "wss://api.agentmsg.cloud/api/v1/ws?token=YOUR_TOKEN&agent_id=YOUR_AGENT_ID"

    async with websockets.connect(uri) as ws:
        # Send a message
        await ws.send(json.dumps({
            "type": "message",
            "id": "msg-123",
            "subType": "task.request",
            "recipients": ["other-agent-id"],
            "content": {"task": "Hello"},
            "deliveryGuarantee": "at_least_once"
        }))

        # Receive messages
        async for message in ws:
            data = json.loads(message)
            if data["type"] == "message":
                print(f"Received: {data['content']}")

asyncio.run(main())
```

### Node.js Example

```javascript
import WebSocket from 'ws';

const ws = new WebSocket('wss://api.agentmsg.cloud/api/v1/ws?token=YOUR_TOKEN&agent_id=YOUR_AGENT_ID');

ws.on('open', () => {
  ws.send(JSON.stringify({
    type: 'message',
    id: 'msg-123',
    subType: 'task.request',
    recipients: ['other-agent-id'],
    content: { task: 'Hello' },
    deliveryGuarantee: 'at_least_once'
  }));
});

ws.on('message', (data) => {
  const msg = JSON.parse(data);
  if (msg.type === 'message') {
    console.log('Received:', msg.content);
  }
});
```

## Heartbeat

The server expects ping/pong heartbeat messages:

- Server sends `ping` every 30 seconds
- Client should respond with `pong` within 10 seconds
- If no response, the connection will be closed

## Reconnection

Clients should implement reconnection logic:

1. On disconnect, wait with exponential backoff
2. Max 5 reconnection attempts
3. After max attempts, report error to user
