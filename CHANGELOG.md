# Changelog

All notable changes to this project will be documented in this file.

# AgentMsg v0.1.0

## Scope

- Version: `0.1.0`
- Comparison: `initial release -> v0.1.0`
- Release artifacts: Node.js tarball, Python sdist, Python wheel, Go SDK source bundle

## Verification

- `make release-check`
- `make build-release-artifacts`

## Features

- `2c502b1` feat: add kustomize environment overlays
- `6121241` feat: publish openapi contract from api server
- `b1044be` feat: add kustomize baseline and align go sdk
- `7d8f322` feat: harden kubernetes deployment baseline
- `03aa03c` feat: validate production configuration at startup
- `6bb8015` feat: add tracing embedded migrations and smoke checks
- `dac182f` feat: add metrics and e2e verification
- `cd61a42` feat: add jwt tracing and audit logging
- `917ecf9` feat: add ack persistence readiness and rate limiting
- `9f7de9e` feat: add dlq persistence and queued delivery flow
- `95b8294` feat: harden core messaging infrastructure

## Fixes

- `af2438b` fix: align sdk clients with implemented api
- `f4ba0b0` fix: gracefully shut down runtime servers
- `5e6468c` fix: enforce tenant isolation in api access
- `d1f387c` fix: resolve build and runtime issues

## Build And Packaging

- `3f8a4a9` build: modernize python sdk packaging

## CI And Release

- `bc87dc5` release: publish sdk packages on root tags
- `1f3043c` release: add go sdk versioning and packaging
- `d7bdded` release: add version bump and notes tooling
- `df363f9` release: automate github artifact publishing
- `ea50cf9` release: add sdk packaging baseline
- `d0d7b7f` ci: make deployments overlay-aware and traceable

## Documentation

- `77ac65a` docs: align public docs with implemented interfaces

## Testing

- `ebe67c0` test: guard openapi contract drift
- `03f074a` test: add node sdk coverage and ci
- `61c3540` test: add sdk coverage for go and python clients

## Other

- `024eea1` Initial commit: AgentMsg - AI Agent Communication Middleware

