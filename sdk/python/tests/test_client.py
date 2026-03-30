import unittest
from unittest.mock import AsyncMock, Mock, patch
from types import SimpleNamespace

from agentmsg import (
    AgentMsgError,
    AuthenticationError,
    Capability,
    Client,
    MessageType,
)


class ClientTests(unittest.IsolatedAsyncioTestCase):
    async def test_connect_checks_health(self):
        mock_http = AsyncMock()
        mock_response = Mock()
        mock_response.raise_for_status = Mock()
        mock_http.get = AsyncMock(return_value=mock_response)
        mock_http.aclose = AsyncMock()

        fake_httpx = SimpleNamespace(AsyncClient=Mock(return_value=mock_http))
        with patch("agentmsg.httpx", new=fake_httpx):
            client = Client(api_key="token", agent_id="agent-1", endpoint="https://example.com")
            await client.connect()

        mock_http.get.assert_awaited_once_with("/health")
        self.assertTrue(client._running)

    async def test_send_message_returns_send_result(self):
        client = Client(api_key="token", agent_id="agent-1")
        response = Mock(status_code=201)
        response.json.return_value = {"messageId": "msg-123", "status": "pending"}
        client._http = AsyncMock()
        client._http.post = AsyncMock(return_value=response)

        result = await client.send_message(
            content={"text": "hello"},
            recipients=["agent-2"],
            message_type=MessageType.GENERIC,
        )

        self.assertEqual("msg-123", result.message_id)
        self.assertEqual("pending", result.status)

    async def test_register_capabilities_requires_existing_agent(self):
        client = Client(api_key="token", agent_id="agent-1")
        response = Mock(status_code=404, text="not found")
        client._http = AsyncMock()
        client._http.put = AsyncMock(return_value=response)

        with self.assertRaises(AgentMsgError):
            await client.register_capabilities(
                [Capability(type="text-generation", description="text")]
            )

    async def test_send_message_unauthorized_maps_error(self):
        client = Client(api_key="token", agent_id="agent-1")
        response = Mock(status_code=401, text="unauthorized")
        client._http = AsyncMock()
        client._http.post = AsyncMock(return_value=response)

        with self.assertRaises(AuthenticationError):
            await client.send_message(content="hello", recipients=["agent-2"])

    async def test_receive_messages_is_not_available(self):
        client = Client(api_key="token", agent_id="agent-1")

        with self.assertRaises(AgentMsgError):
            await client.receive_messages()
