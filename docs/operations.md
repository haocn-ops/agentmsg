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

The manifests now include:

- Rolling update strategies
- Pod disruption budgets
- Startup, readiness, and liveness probes
- Non-root runtime and restricted container security settings
- A `kustomize` entrypoint for consistent rollout order
- Baseline `NetworkPolicy` objects

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
