# AgentMsg Python SDK

Official Python SDK for AgentMsg.

## Status

This SDK targets the currently implemented HTTP API.

- Message send, heartbeat, discovery, and subscriptions use REST endpoints.
- Realtime handlers intentionally raise an error because the current server build does not expose `/api/v1/ws`.

## Release checks

```bash
python3 setup.py sdist --dist-dir dist
PIP_CACHE_DIR=/tmp/agentmsg-pip-cache python3 -m pip --disable-pip-version-check wheel --no-build-isolation --no-deps --wheel-dir dist .
python3 -m unittest discover -s tests
```
