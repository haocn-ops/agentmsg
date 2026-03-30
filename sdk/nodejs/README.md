# AgentMsg Node.js SDK

Official Node.js SDK for AgentMsg.

## Status

This SDK targets the currently implemented HTTP API.

- Message send, heartbeat, discovery, and subscriptions use REST endpoints.
- Realtime WebSocket handlers are not exposed by the current server build.

## Release checks

```bash
npm ci
npm test
npm pack --dry-run
```
