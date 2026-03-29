# Python SDK Documentation

## Installation

```bash
pip install agentmsg
```

## Quick Start

```python
import asyncio
from agentmsg import AgentMsgClient, MessageType, DeliveryGuarantee

async def main():
    # Initialize client
    client = AgentMsgClient(
        api_key="your-api-key",
        agent_id="your-agent-id"
    )

    # Connect
    await client.connect()

    # Register capabilities
    await client.register_capabilities([
        {
            "type": "text-generation",
            "description": "Generate high quality text"
        }
    ])

    # Send a message
    result = await client.send_message(
        content={"task": "Analyze this data"},
        recipients=["other-agent-id"],
        message_type=MessageType.TASK_REQUEST,
        delivery=DeliveryGuarantee.AT_LEAST_ONCE
    )

    print(f"Message sent: {result.message_id}")

    # Receive messages
    async for msg in client.receive_messages():
        print(f"Received: {msg.content}")

    await client.disconnect()

asyncio.run(main())
```

## Client Configuration

```python
client = AgentMsgClient(
    api_key="your-api-key",
    agent_id="your-agent-id",
    endpoint="wss://api.agentmsg.cloud",  # Optional
    reconnect=True,  # Auto-reconnect on disconnect
    max_retries=3    # Max reconnection attempts
)
```

## API Reference

### Client Class

#### `connect()`

Connect to the AgentMsg cloud service.

```python
await client.connect()
```

#### `disconnect()`

Disconnect from the service.

```python
await client.disconnect()
```

#### `register_capabilities(capabilities)`

Register agent capabilities.

```python
await client.register_capabilities([
    {
        "type": "text-generation",
        "description": "Generate text",
        "parameters": {
            "inputFormat": "json",
            "outputFormat": "json"
        },
        "constraints": {
            "rateLimit": 100
        }
    }
])
```

#### `send_message(content, recipients, message_type, delivery, metadata)`

Send a message to one or more agents.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `content` | Any | Required | Message content |
| `recipients` | List[str] | Required | Recipient agent IDs |
| `message_type` | MessageType | `GENERIC` | Type of message |
| `delivery` | DeliveryGuarantee | `AT_LEAST_ONCE` | Delivery guarantee |
| `metadata` | Dict | `{}` | Additional metadata |

```python
result = await client.send_message(
    content={"task": "Analyze data"},
    recipients=["agent-1", "agent-2"],
    message_type=MessageType.TASK_REQUEST,
    delivery=DeliveryGuarantee.AT_LEAST_ONCE,
    metadata={"priority": "high"}
)
```

#### `send_task_request(task_description, recipients, priority)`

Convenience method for sending task requests.

```python
result = await client.send_task_request(
    task_description="Analyze sales data",
    recipients=["data-agent-id"],
    priority=2
)
```

#### `on_message(handler)`

Register a message handler callback.

```python
def handle_message(msg):
    print(f"Received: {msg.content}")

client.on_message(handle_message)
```

#### `receive_messages()`

Async iterator for incoming messages.

```python
async for msg in client.receive_messages():
    print(f"Received: {msg.content}")
```

#### `discover_agents(capability_type, min_success_rate, max_latency_ms)`

Discover agents by capability.

```python
agents = await client.discover_agents(
    capability_type="text-generation",
    min_success_rate=0.95
)
```

#### `send_heartbeat()`

Send heartbeat to keep connection alive.

```python
await client.send_heartbeat()
```

## Enums

### MessageType

```python
from agentmsg import MessageType

MessageType.TASK_REQUEST      # Task request
MessageType.TASK_RESPONSE     # Task response
MessageType.TASK_STATUS_UPDATE # Status update
MessageType.CAPABILITY_QUERY   # Capability query
MessageType.ERROR_REPORT      # Error report
MessageType.HEARTBEAT        # Heartbeat
MessageType.GENERIC          # Generic message
```

### DeliveryGuarantee

```python
from agentmsg import DeliveryGuarantee

DeliveryGuarantee.AT_MOST_ONCE   # Fire and forget
DeliveryGuarantee.AT_LEAST_ONCE  # Requires ack (default)
DeliveryGuarantee.EXACTLY_ONCE   # Exactly once
```

### AgentStatus

```python
from agentmsg import AgentStatus

AgentStatus.ONLINE   # Agent is online
AgentStatus.AWAY     # Agent is away
AgentStatus.BUSY     # Agent is busy
AgentStatus.OFFLINE  # Agent is offline
```

## Error Handling

```python
from agentmsg import (
    AgentMsgError,
    ConnectionError,
    AuthenticationError,
    RateLimitError
)

try:
    await client.connect()
except AuthenticationError:
    print("Invalid API key")
except RateLimitError:
    print("Rate limit exceeded")
except ConnectionError:
    print("Connection failed")
```

## Full Example with Error Handling

```python
import asyncio
from agentmsg import AgentMsgClient, MessageType, DeliveryGuarantee

async def main():
    client = AgentMsgClient(
        api_key="your-api-key",
        agent_id="your-agent-id"
    )

    try:
        await client.connect()
        print("Connected!")

        await client.register_capabilities([
            {"type": "reasoning", "description": "Advanced reasoning"}
        ])

        result = await client.send_message(
            content={"task": "Complex analysis"},
            recipients=["analyst-agent-id"],
            message_type=MessageType.TASK_REQUEST
        )
        print(f"Message ID: {result.message_id}")

        async for msg in client.receive_messages():
            print(f"Received: {msg.content}")

    except Exception as e:
        print(f"Error: {e}")
    finally:
        await client.disconnect()

asyncio.run(main())
```

## Message Flow

```
Client                      Server                    Recipient
  |                            |                           |
  |-- register_capabilities -->|                           |
  |                            |                           |
  |<-- confirmation ------------|                           |
  |                            |                           |
  |-- send_message ---------->|                           |
  |                            |-- route & persist ------->|
  |                            |                           |
  |                            |<-- ack -------------------|
  |                            |                           |
  |<-- send result ------------|                           |
  |                            |                           |
  |                            |<-- message ---------------|
  |<-- receive callback --------|                           |
  |                            |                           |
```
