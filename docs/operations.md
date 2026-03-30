# Operations Guide

## Deployment Baseline

- Apply the namespace, config, and secret objects before the workloads.
- Replace `JWT_SECRET`, `DATABASE_URL`, and `REDIS_URL` in `deployments/k8s/namespace.yaml` before deploying to production.
- If `OTEL_ENABLED=true`, keep `OTEL_EXPORTER_OTLP_ENDPOINT` pointed at a reachable collector such as `http://otel-collector:4318`.
- Keep at least one PostgreSQL backup and one Redis persistence snapshot strategy outside the cluster rollout workflow.

## Kubernetes Rollout

```bash
kubectl apply -k deployments/k8s
```

Environment overlays:

```bash
kubectl apply -k deployments/k8s/overlays/staging
kubectl apply -k deployments/k8s/overlays/production
```

The manifests now include:

- Rolling update strategies
- Pod disruption budgets
- Startup, readiness, and liveness probes
- Non-root runtime and restricted container security settings
- A `kustomize` entrypoint for consistent rollout order
- Baseline `NetworkPolicy` objects
- Staging and production overlays

## GitHub Deploy Secrets

For GitHub Actions based deployments, configure these environment secrets:

- `STAGING_KUBECONFIG`: raw kubeconfig content for the staging cluster
- `STAGING_KUBECONFIG_CONTEXT`: optional staging kubeconfig context override
- `PRODUCTION_KUBECONFIG`: raw kubeconfig content for the production cluster
- `PRODUCTION_KUBECONFIG_CONTEXT`: optional production kubeconfig context override
- `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`: registry credentials for the published runtime images
- `NPM_TOKEN`: npm access token for publishing `@agentmsg/sdk`
- `PYPI_API_TOKEN`: PyPI API token for publishing `agentmsg`

The CI workflow publishes both `latest` and commit-SHA image tags. Staging and production deploys render the appropriate overlay and pin workloads to the commit SHA so every rollout is traceable and rollback-friendly.
The release workflow always builds GitHub release assets and checksum files. On root tags like `v0.1.0`, it also publishes the Node.js and Python SDKs when the registry secrets are configured.

If you use the bundled `allow-api-gateway-ingress` policy, label the ingress controller namespace with:

```bash
kubectl label namespace ingress-nginx agentmsg.ingress-access=true
```

If Prometheus runs outside the `agentmsg` namespace, label its namespace too:

```bash
kubectl label namespace monitoring agentmsg.metrics-access=true
```

## Preflight Checks

Before a production rollout, verify:

- `kubectl get pods -n agentmsg`
- `kubectl get hpa -n agentmsg`
- `kubectl get pdb -n agentmsg`
- `kubectl describe ingress agentmsg-ingress -n agentmsg`
- `kubectl logs deploy/api-gateway -n agentmsg --tail=100`
- `kubectl logs deploy/message-engine -n agentmsg --tail=100`

## Rollback

```bash
kubectl rollout undo deployment/api-gateway -n agentmsg
kubectl rollout undo deployment/message-engine -n agentmsg
```

After rollback, re-check `/ready` and metrics scraping before reopening traffic.
