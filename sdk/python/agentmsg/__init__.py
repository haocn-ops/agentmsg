"""
AgentMsg Python SDK
~~~~~~~~~~~~~~~~~~

A simple, reliable messaging SDK for AI Agent communication.
"""

__version__ = "0.1.0"

from typing import Any, Dict, List, Optional, Callable, AsyncIterator
from dataclasses import dataclass, field
from enum import Enum
import asyncio
import json
import logging
import uuid
import websockets
import httpx

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
        endpoint: str = "wss://api.agentmsg.cloud",
        reconnect: bool = True,
        max_retries: int = 3,
    ):
        self.api_key = api_key
        self.agent_id = agent_id
        self.endpoint = endpoint.rstrip("/")
        self.reconnect = reconnect
        self.max_retries = max_retries

        self._http: Optional[httpx.AsyncClient] = None
        self._ws: Optional[websockets.WebSocketClientProtocol] = None
        self._running = False
        self._message_handlers: List[Callable] = []
        self._ack_handlers: Dict[str, asyncio.Future] = {}

    async def __aenter__(self):
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.disconnect()

    async def connect(self) -> None:
        """Connect to AgentMsg cloud."""
        self._http = httpx.AsyncClient(
            base_url=self.endpoint,
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "X-Agent-ID": self.agent_id,
            },
            timeout=30.0,
        )

        ws_url = f"wss://{self.endpoint.replace('https://', '').replace('http://', '')}/api/v1/ws?token={self.api_key}&agent_id={self.agent_id}"

        try:
            self._ws = await websockets.connect(
                ws_url,
                ping_interval=30,
                ping_timeout=10,
            )
            logger.info("WebSocket connected")
            self._running = True
            asyncio.create_task(self._message_loop())
        except Exception as e:
            logger.error(f"WebSocket connection failed: {e}")
            raise ConnectionError(f"Failed to connect: {e}")

    async def disconnect(self) -> None:
        """Disconnect from AgentMsg cloud."""
        self._running = False
        if self._ws:
            await self._ws.close()
            self._ws = None
        if self._http:
            await self._http.aclose()
            self._http = None
        logger.info("Disconnected from AgentMsg")

    async def _message_loop(self) -> None:
        """Main WebSocket message handling loop."""
        while self._running and self._ws:
            try:
                message = await self._ws.recv()
                await self._handle_message(json.loads(message))
            except websockets.ConnectionClosed:
                logger.warning("WebSocket connection closed")
                if self.reconnect:
                    await self._reconnect()
                else:
                    break
            except Exception as e:
                logger.error(f"Error in message loop: {e}")

    async def _reconnect(self) -> None:
        """Reconnect with exponential backoff."""
        for attempt in range(self.max_retries):
            delay = 1.0 * (2 ** attempt)
            logger.info(f"Reconnecting in {delay}s (attempt {attempt + 1})")
            await asyncio.sleep(delay)
            try:
                await self.connect()
                logger.info("Reconnected successfully")
                return
            except Exception as e:
                logger.error(f"Reconnect failed: {e}")
        raise ConnectionError("Max reconnection attempts reached")

    async def _handle_message(self, data: Dict) -> None:
        """Handle incoming WebSocket message."""
        msg_type = data.get("type")
        if msg_type == "message":
            for handler in self._message_handlers:
                asyncio.create_task(handler(data))
        elif msg_type == "ack":
            if data.get("id") in self._ack_handlers:
                self._ack_handlers[data["id"]].set_result(data)

    async def register_capabilities(self, capabilities: List[Capability]) -> AgentIdentity:
        """Register agent capabilities."""
        response = await self._http.post(
            f"/api/v1/agents/{self.agent_id}/capabilities",
            json=[cap.__dict__ for cap in capabilities],
        )
        if response.status_code == 401:
            raise AuthenticationError("Invalid API key")
        elif response.status_code == 429:
            raise RateLimitError("Rate limit exceeded")
        elif response.status_code != 200:
            raise AgentMsgError(f"Failed to register: {response.text}")
        return AgentIdentity(**response.json())

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
        """Send a message to agents."""
        message = Message(
            id=str(uuid.uuid4()),
            conversation_id=str(uuid.uuid4()),
            message_type=message_type,
            sender_id=self.agent_id,
            recipients=recipients,
            content=content,
            metadata=metadata or {},
            delivery_guarantee=delivery,
        )

        ws_message = message.to_ws_dict()
        await self._ws.send(json.dumps(ws_message))

        return SendResult(message_id=message.id, status="sent")

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
        """Register a message handler."""
        self._message_handlers.append(handler)

    async def receive_messages(self) -> AsyncIterator[Message]:
        """Async iterator for incoming messages."""
        queue: asyncio.Queue = asyncio.Queue()

        async def handler(data: Dict):
            await queue.put(Message.from_dict(data))

        self.on_message(handler)
        while self._running:
            try:
                yield await asyncio.wait_for(queue.get(), timeout=1.0)
            except asyncio.TimeoutError:
                continue

    async def discover_agents(
        self,
        capability_type: Optional[str] = None,
        min_success_rate: Optional[float] = None,
    ) -> List[AgentIdentity]:
        """Discover agents by capability."""
        params = {}
        if capability_type:
            params["capabilityType"] = capability_type
        if min_success_rate is not None:
            params["minSuccessRate"] = min_success_rate

        response = await self._http.get(
            "/api/v1/discovery/query",
            params=params,
        )
        if response.status_code != 200:
            raise AgentMsgError(f"Discovery failed: {response.text}")

        data = response.json()
        return [AgentIdentity(**a) for a in data.get("agents", [])]
