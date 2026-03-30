# AgentMsg Python SDK

Official Python SDK for AgentMsg.

## Status

This SDK targets the currently implemented HTTP API.

- Message send, heartbeat, discovery, and subscriptions use REST endpoints.
- Realtime handlers intentionally raise an error because the current server build does not expose `/api/v1/ws`.

## Release checks

```bash
rm -rf build dist ./*.egg-info
python3 setup.py sdist --dist-dir dist
rm -rf build ./*.egg-info
python3 setup.py bdist_wheel --dist-dir dist
python3 -m unittest discover -s tests
```
