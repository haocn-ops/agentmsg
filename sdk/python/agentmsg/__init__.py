"""
AgentMsg Python SDK
~~~~~~~~~~~~~~~~~~

A simple, reliable messaging SDK for AI Agent communication.
"""

__version__ = "0.1.0"

from typing import Any, Dict, List, Optional, Callable, AsyncIterator
from dataclasses import dataclass, field
from enum import Enum
import logging

try:
    import httpx
except ImportError:  # pragma: no cover - exercised in dependency-light environments
    httpx = None

logger = logging.getLogger(__name__)


class MessageType(Enum):
    TASK_REQUEST = "task.request"
    TASK_RESPONSE = "task.response"
    TASK_STATUS_UPDATE = "task.status-update"
    TASK_DELEGATE = "task.delegate"
    CAPABILITY_QUERY = "capability.query"
    CAPABILITY_ADVERT = "capability.advertise"
    ERROR_REPORT = "error.report"
    HEARTBEAT = "heartbeat"
    GENERIC = "generic"


class DeliveryGuarantee(Enum):
    AT_MOST_ONCE = "at_most_once"
    AT_LEAST_ONCE = "at_least_once"
    EXACTLY_ONCE = "exactly_once"


class AgentStatus(Enum):
    ONLINE = "online"
    AWAY = "away"
    BUSY = "busy"
    OFFLINE = "offline"


@dataclass
class Capability:
    type: str
    description: str
    examples: Optional[List[str]] = None
    parameters: Optional[Dict] = None
    constraints: Optional[Dict] = None


@dataclass
class AgentIdentity:
    agent_id: str
    public_key: str
    name: Optional[str] = None
    version: Optional[str] = None
    provider: Optional[str] = None
    capabilities: List[Capability] = field(default_factory=list)


@dataclass
class Message:
    id: str
    conversation_id: str
    message_type: MessageType
    sender_id: str
    content: Any
    recipients: List[str] = field(default_factory=list)
    metadata: Dict = field(default_factory=dict)
    delivery_guarantee: DeliveryGuarantee = DeliveryGuarantee.AT_LEAST_ONCE

    def to_ws_dict(self) -> Dict:
        return {
            "type": "message",
            "id": self.id,
            "conversationId": self.conversation_id,
            "subType": self.message_type.value,
            "recipients": self.recipients,
            "content": self.content,
            "metadata": self.metadata,
            "deliveryGuarantee": self.delivery_guarantee.value,
        }

    @classmethod
    def from_dict(cls, data: Dict) -> "Message":
        return cls(
            id=data["id"],
            conversation_id=data["conversationId"],
            message_type=MessageType(data.get("subType", "generic")),
            sender_id=data["sender"]["agentId"],
            content=data["content"],
            recipients=data.get("recipients", []),
            metadata=data.get("metadata", {}),
        )


@dataclass
class SendResult:
    message_id: str
    status: str
    delivered_at: Optional[int] = None


class AgentMsgError(Exception):
    pass


class ConnectionError(AgentMsgError):
    pass


class AuthenticationError(AgentMsgError):
    pass


class RateLimitError(AgentMsgError):
    pass


class Client:
    """Main client for AgentMsg SDK."""

    def __init__(
        self,
        api_key: str,
        agent_id: str,
        endpoint: str = "https://api.agentmsg.cloud",
        name: Optional[str] = None,
        version: str = "0.1.0",
        provider: str = "agentmsg-python-sdk",
        reconnect: bool = True,
        max_retries: int = 3,
    ):
        self.api_key = api_key
        self.agent_id = agent_id
        self.endpoint = endpoint.rstrip("/")
        self.name = name or agent_id
        self.version = version
        self.provider = provider
        self.reconnect = reconnect
        self.max_retries = max_retries

        self._http: Optional[Any] = None
        self._running = False
        self._message_handlers: List[Callable] = []

    async def __aenter__(self):
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.disconnect()

    async def connect(self) -> None:
        """Initialize the HTTP client for AgentMsg."""
        if httpx is None:
            raise AgentMsgError("httpx is required to use the Python SDK")
        http_endpoint = self.endpoint.replace("wss://", "https://").replace("ws://", "http://")
        self._http = httpx.AsyncClient(
            base_url=http_endpoint,
            headers={
                "Authorization": f"Bearer {self.api_key}",
            },
            timeout=30.0,
        )
        try:
            response = await self._http.get("/health")
            response.raise_for_status()
            self._running = True
            logger.info("HTTP client connected")
        except Exception as e:
            logger.error(f"HTTP initialization failed: {e}")
            raise ConnectionError(f"Failed to connect: {e}")

    async def disconnect(self) -> None:
        """Disconnect from AgentMsg cloud."""
        self._running = False
        if self._http:
            await self._http.aclose()
            self._http = None
        logger.info("Disconnected from AgentMsg")

    async def register_capabilities(self, capabilities: List[Capability]) -> AgentIdentity:
        """Update capabilities for an existing agent identity."""
        response = await self._http.put(
            f"/api/v1/agents/{self.agent_id}",
            json={
                "name": self.name,
                "version": self.version,
                "provider": self.provider,
                "capabilities": [cap.__dict__ for cap in capabilities],
            },
        )
        if response.status_code == 401:
            raise AuthenticationError("Invalid token")
        elif response.status_code == 404:
            raise AgentMsgError("Agent must already exist before capabilities can be updated")
        elif response.status_code == 429:
            raise RateLimitError("Rate limit exceeded")
        elif response.status_code != 200:
            raise AgentMsgError(f"Failed to register: {response.text}")
        payload = response.json()
        return AgentIdentity(
            agent_id=payload["id"],
            public_key=payload.get("publicKey", ""),
            name=payload.get("name"),
            version=payload.get("version"),
            provider=payload.get("provider"),
            capabilities=[
                Capability(**capability) for capability in payload.get("capabilities", [])
            ],
        )

    async def send_heartbeat(self) -> None:
        """Send heartbeat."""
        await self._http.post(f"/api/v1/agents/{self.agent_id}/heartbeat")

    async def send_message(
        self,
        content: Any,
        recipients: List[str],
        message_type: MessageType = MessageType.GENERIC,
        delivery: DeliveryGuarantee = DeliveryGuarantee.AT_LEAST_ONCE,
        metadata: Optional[Dict] = None,
    ) -> SendResult:
        """Send a message to agents using the REST API."""
        response = await self._http.post(
            "/api/v1/messages",
            json={
                "messageType": message_type.value,
                "recipients": recipients,
                "content": content,
                "metadata": metadata or {},
                "deliveryGuarantee": delivery.value,
            },
        )
        if response.status_code == 401:
            raise AuthenticationError("Invalid token")
        if response.status_code == 429:
            raise RateLimitError("Rate limit exceeded")
        if response.status_code != 201:
            raise AgentMsgError(f"Failed to send message: {response.text}")

        payload = response.json()
        return SendResult(message_id=payload["messageId"], status=payload["status"])

    async def send_task_request(
        self,
        task_description: str,
        recipients: Optional[List[str]] = None,
        priority: int = 0,
    ) -> SendResult:
        """Send a task request."""
        return await self.send_message(
            content={"description": task_description},
            recipients=recipients or [],
            message_type=MessageType.TASK_REQUEST,
            delivery=DeliveryGuarantee.AT_LEAST_ONCE,
            metadata={"priority": priority},
        )

    def on_message(self, handler: Callable[[Message], None]) -> None:
        """Realtime hooks are unavailable until the server exposes websocket support."""
        _ = handler
        raise AgentMsgError("Realtime message handlers are not available in the current server build")

    async def receive_messages(self) -> AsyncIterator[Message]:
        """Placeholder until realtime transport is available on the server."""
        raise AgentMsgError("Realtime message streaming is not available in the current server build")

    async def discover_agents(
        self,
        capability_type: Optional[str] = None,
        min_success_rate: Optional[float] = None,
    ) -> List[AgentIdentity]:
        """Discover agents by capability."""
        payload: Dict[str, Any] = {
            "capabilities": [capability_type] if capability_type else [],
            "tags": {},
        }
        if min_success_rate is not None:
            payload["tags"]["minSuccessRate"] = str(min_success_rate)

        response = await self._http.post(
            "/api/v1/discovery/query",
            json=payload,
        )
        if response.status_code != 200:
            raise AgentMsgError(f"Discovery failed: {response.text}")

        data = response.json()
        return [
            AgentIdentity(
                agent_id=agent["id"],
                public_key=agent.get("publicKey", ""),
                name=agent.get("name"),
                version=agent.get("version"),
                provider=agent.get("provider"),
                capabilities=[
                    Capability(**capability) for capability in agent.get("capabilities", [])
                ],
            )
            for agent in data.get("agents", [])
        ]
