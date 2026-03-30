# AgentMsg Go SDK

Official Go SDK for AgentMsg.

## Install

```bash
go get github.com/haocn-ops/agentmsg/sdk/go/agentmsg@latest
```

## Status

This SDK targets the currently implemented HTTP API.

- Message send, heartbeat, agent management, and discovery use REST endpoints.
- Realtime receive is not exposed by the current server build.

## Release notes

Because this module lives in a repository subdirectory, Go module tags should use the `sdk/go/agentmsg/` prefix, for example `sdk/go/agentmsg/v0.1.0`.
