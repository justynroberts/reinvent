# 03: Kubernetes cluster, demo workloads and the Automation Runner

The workflows act on a real cluster, so you need one. If your sandbox has no Kubernetes cluster, run **kind** on your laptop. The internal page *UA Runner Kind Cluster Management Script* covers the same ground; use it if it has moved on from what's written here.

## 1. Cluster

```bash
./trigger cluster up        # menu option 4: creates the kind cluster if needed, applies the workloads
```

The cluster has a single node, `reinvent-control-plane`, which is the "same node" in the story. The payloads use this name (`payloads/scenario.json` → `vars.node`). If you use a different cluster, update `vars.node`, `vars.cluster` and `vars.namespace` there.

## 2. Demo workloads

```bash
# done by ./trigger cluster up: applies k8s/healthcare.yaml and k8s/remediation-toolbox.yaml
kubectl -n healthcare get pods -o wide
kubectl -n healthcare logs deploy/claims-reconciliation -f   # watch the "leak"
```

| Object | Purpose |
|---|---|
| `Deployment/claims-reconciliation` | Python process that leaks 5 MB every 10 s up to 300 MB, then logs a blocked worker. **No memory limit**, which is the bug. Label `app=claims-reconciliation`, container `app`. |
| `Deployment/patient-portal`, `Deployment/scheduling` | nginx neighbours on the same node, for the pod list |
| `Pod/claims-toolbox` + SA/Role | kubectl toolbox allowed to `patch` Deployments in `healthcare` only. Used by the remediation pod command. |
| `k8s/claims-reconciliation-adhoc-pod.yaml` | Replacement pod spec, with a memory limit, for the *Pod Created Adhoc* variant |

The leak is capped so a laptop survives it. Real node pressure and evictions are **not** reproduced; that part of the story comes from the alert payloads. The pod list, delete, recreate and memory-limit patch are all real.

## 3. PD Automation Runner

Requires the *PD Automation Runner* EA flag.

1. Automation → Incident Workflows → **Automation Connectors** → **Self-hosted Runners** → create a runner. Copy the runner ID, secret and API token. They are shown once.
2. Deploy the runner pod using the manifest from the **PD Automation Runner KB**. A known-good reference shape is in `~/work/unifiedrunner/kubernetes.yaml`: namespace and SA `unified-automation-runner`, a read-mostly ClusterRole with `pods` create/delete and `pods/exec`. Put the credentials in the Secret. Don't commit them.
3. `RUNNER_CLOUD_URL` must match the region: `https://api.pagerduty.com` or `https://api.eu.pagerduty.com`.
4. Wait for **Healthy** under Self-hosted Runners.

### RBAC: what the remediation needs

| Step | Needs | Granted by |
|---|---|---|
| List claims-reconciliation pods | `pods` get/list | Runner ClusterRole (KB) |
| Delete Pod | `pods` delete | Runner ClusterRole (KB) |
| Create Pod (adhoc variant) | `pods` create | Runner ClusterRole (KB) |
| Pod Command → set memory limit | `pods/exec` create (runner) + `deployments` patch (toolbox SA) | Runner ClusterRole + `k8s/remediation-toolbox.yaml` |

The runner itself is never given `patch` on Deployments. It execs into `claims-toolbox`, and the toolbox's namespace-scoped SA does the patch.

## Reset the cluster between demos

```bash
./trigger cluster reset   # menu option 8: deletes and recreates the healthcare workloads (removes the applied memory limit)
```
