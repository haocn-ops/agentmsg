# AgentMsg Python SDK

Official Python SDK for AgentMsg.

## Status

This SDK targets the currently implemented HTTP API.

- Message send, heartbeat, discovery, and subscriptions use REST endpoints.
- Realtime handlers intentionally raise an error because the current server build does not expose `/api/v1/ws`.

## Release checks

```bash
python3 -m pip install build
rm -rf build dist ./*.egg-info
python3 -m build --sdist --wheel --outdir dist --no-isolation
python3 -m unittest discover -s tests
```
