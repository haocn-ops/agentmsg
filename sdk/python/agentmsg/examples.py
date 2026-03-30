"""
AgentMsg Python SDK - Usage Examples
"""

import asyncio
import os
from agentmsg import Client, MessageType, DeliveryGuarantee, Capability


async def basic_example():
    """Basic messaging example."""
    api_key = os.getenv("AGENTMSG_API_KEY", "test-api-key")
    agent_id = os.getenv("AGENTMSG_AGENT_ID", "test-agent-id")

    async with Client(api_key=api_key, agent_id=agent_id) as client:
        await client.register_capabilities([
            Capability(
                type="text-generation",
                description="Generate text content"
            )
        ])

        result = await client.send_message(
            content={"text": "Hello, World!"},
            recipients=["recipient-agent-id"],
            message_type=MessageType.GENERIC,
            delivery=DeliveryGuarantee.AT_LEAST_ONCE
        )
        print(f"Message sent: {result.message_id}, status: {result.status}")


async def task_request_example():
    """Task request example."""
    api_key = os.getenv("AGENTMSG_API_KEY", "test-api-key")
    agent_id = os.getenv("AGENTMSG_AGENT_ID", "test-agent-id")

    async with Client(api_key=api_key, agent_id=agent_id) as client:
        result = await client.send_task_request(
            task_description="Analyze this data and return insights",
            recipients=["data-analysis-agent-id"],
            priority=1
        )
        print(f"Task sent: {result.message_id}")


async def capability_discovery_example():
    """Capability discovery example."""
    api_key = os.getenv("AGENTMSG_API_KEY", "test-api-key")
    agent_id = os.getenv("AGENTMSG_AGENT_ID", "test-agent-id")

    async with Client(api_key=api_key, agent_id=agent_id) as client:
        agents = await client.discover_agents(
            capability_type="text-generation",
            min_success_rate=0.95
        )
        print(f"Found {len(agents)} agents with text-generation capability")
        for agent in agents:
            print(f"  - {agent.name} ({agent.agent_id})")


async def exactly_once_example():
    """The current server build does not expose realtime streaming yet."""
    api_key = os.getenv("AGENTMSG_API_KEY", "test-api-key")
    agent_id = os.getenv("AGENTMSG_AGENT_ID", "test-agent-id")

    async with Client(api_key=api_key, agent_id=agent_id) as client:
        try:
            async for message in client.receive_messages():
                print(f"Received (exactly-once): {message.content}")
        except Exception as exc:
            print(f"Realtime receive is not available yet: {exc}")


if __name__ == "__main__":
    asyncio.run(basic_example())
