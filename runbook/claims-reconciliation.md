# Runbook: claims-reconciliation

| | |
|---|---|
| **Service** | `claims-reconciliation` (PagerDuty technical service) |
| **Supports** | `patient-portal` (business service) |
| **Runs on** | Kubernetes, namespace `healthcare`, Deployment `claims-reconciliation`, container `app` |
| **Co-located with** | `scheduling`, `patient-portal` pods (same node) |
| **Watch** | Claims acknowledgements (X12 999/277CA) have contractual deadlines, so check the `claims.submissions.837` backlog early. |

## What it does

Consumes claim submissions from the `claims.submissions.837` queue, parses them with `x12-claims-parser`, and reconciles them against payer remittance (835). It does not serve user traffic, but it shares a node with services that do.

## Known failure: memory leak → node memory pressure

**Signature**

- Container working set grows linearly after a rollout and never plateaus.
- The container has **no memory limit**, so it is not OOM-killed. It grows until the node hits `MemoryPressure`.
- The kubelet evicts *other* pods on the node first (`patient-portal`, `scheduling`), so customer-facing workloads fail before claims-reconciliation does.
- The worker thread blocks: heartbeat stops, queue consumer stops acking, and `claims.submissions.837` backs up.

**Most likely cause**

A dependency change in the most recent rollout. Check change events on the service first. `x12-claims-parser` 5.x pools segment buffers per payer connection and does not release them. This was introduced in `v2.14.0`.

## Diagnose

```bash
# Which pod, which node, how long since the rollout
kubectl -n healthcare get pods -l app=claims-reconciliation -o wide
kubectl -n healthcare rollout history deployment/claims-reconciliation

# Memory (requires metrics-server)
kubectl -n healthcare top pod -l app=claims-reconciliation
kubectl describe node <node> | grep -A5 Conditions

# Confirm there is no limit
kubectl -n healthcare get deployment claims-reconciliation \
  -o jsonpath='{.spec.template.spec.containers[0].resources}'

# Hung worker
kubectl -n healthcare logs deploy/claims-reconciliation --tail=50 | grep -E 'blocked|heartbeat'

# Collateral
kubectl -n healthcare get pods --field-selector=status.phase=Failed
```

Confirm the leak if **all** of these are true:

- Memory has been growing since the last rollout.
- No memory limit is set.
- Co-located pods were evicted for `MemoryPressure`.

## Remediate (approved fix)

**Restart the pod and apply a memory limit.** Both steps are needed. A restart alone frees the memory, but the leak will take the node down again in about an hour.

### A. Pod created through a Deployment (normal case)

PagerDuty workflow: **Kubernetes Remediation** → *Pod Created through Deployment*.

```bash
# 1. Restart: delete the leaking pod; the Deployment recreates it
kubectl -n healthcare delete pod -l app=claims-reconciliation

# 2. Wait for the replacement
kubectl -n healthcare rollout status deployment/claims-reconciliation --timeout=120s

# 3. Apply a memory limit (run from the claims-toolbox pod, which may patch Deployments)
kubectl -n healthcare set resources deployment/claims-reconciliation -c app \
  --requests=memory=256Mi --limits=memory=512Mi
```

### B. Pod created ad hoc (no Deployment)

PagerDuty workflow: **Kubernetes Remediation** → *Pod Created Adhoc*.

```bash
kubectl -n healthcare delete pod claims-reconciliation
sleep 15
kubectl apply -f k8s/claims-reconciliation-adhoc-pod.yaml   # same spec, with memory limit
```

## Verify

- [ ] New `claims-reconciliation` pod is `Running` and `Ready`. Its limits show `memory: 512Mi`.
- [ ] Node `MemoryPressure=False`.
- [ ] `scheduling` and `patient-portal` pods are rescheduled and `Ready 1/1`.
- [ ] Queue depth on `claims.submissions.837` is falling and the consumer is acking.
- [ ] No new `KubePodMemoryNearNodeAllocatable` or `KubeNodeMemoryPressure` alerts for 15 minutes.

## Follow-up (not during the incident)

- Pin `x12-claims-parser` to `4.2.1` or roll back to `v2.13.4` until upstream fixes buffer pooling.
- Add a `LimitRange` to `healthcare` so no container ships without a memory limit.
- Add an alert on working-set growth rate, not only absolute usage.
