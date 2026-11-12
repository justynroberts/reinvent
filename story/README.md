# The story: a quiet library upgrade takes down patient care

> Forty-seven minutes ago a routine dependency bump shipped to `claims-reconciliation`. Nobody noticed. Now patients can't log in, clinics can't book appointments, and insurance claims are piling up against a compliance deadline. The one pod responsible never crashed, so nothing told anyone.

Original brief: [brief.md](brief.md). Setup: [../instructions/](../instructions/).

## The cast

| Who / what | Role in the story |
|---|---|
| `claims-reconciliation` | Back-office technical service. Not customer-facing. The culprit. |
| `scheduling` | Technical service (appointment booking) on the same Kubernetes node. Its pods get evicted. The collateral damage. |
| `patient-portal` | The customer-facing **business service**. Depends on both technical services. What patients see go down. |
| **Reinvent Patient** | Global Event Orchestration. Every alert enters here and is routed by its `service` field. |
| `x12-claims-parser 4.2.1 → 5.0.0` | The library upgrade shipped in `v2.14.0`. It pools buffers and never frees them. |
| **Paige** | SRE Agent at level 1 of the escalation policy. Investigates first. |
| **You** | The human responder at level 2. You approve, and Paige and the workflows do the work. |

## Why this incident is hard without PagerDuty

- **The failing pod is the one that stays up.** With no memory limit, it is never OOM-killed. The kubelet evicts its neighbours instead, so the first alerts point at the wrong services.
- **The cause is 47 minutes old** and lives in a deploy pipeline, not in the alert.
- **The impact is business-level** (patients, clinics, compliance), but the signals are infrastructure-level (node pressure, evictions, queue depth).

## Beats

Each beat lists what to say, what's on screen, and the setup it depends on.

### 0. Before: the console (30s)

- **Show:** the left nav vs the top nav, then the saved console view with the **Paige** column and the **Alerts** field.
- **Say:** "This is where an on-call engineer lives. Watch the Paige column."
- *Depends on:* the saved console view ([01](../instructions/01-platform.md)).

### 1. A change nobody worried about

- **Trigger:** `./trigger`, then **RUN THE DEMO**. Two **change events** arrive first, both backdated:
  - `scheduling` **v3.8.2**, 3 hours ago: a config-only cache tweak. It's the red herring.
  - `claims-reconciliation` **v2.14.0**, 47 minutes ago: *x12-claims-parser 4.2.1 → 5.0.0*.
- **Say:** "CI shipped a dependency bump. Green build, no alerts. PagerDuty records it anyway."
- *Depends on:* a change-event integration on each technical service (Terraform).

### 2. Noise is dropped, signal escalates

- An **info** probe-latency event arrives and is **suppressed** by Event Orchestration. Nobody is paged.
- Then the real alerts arrive at one front door, the **Reinvent Patient** orchestration, all mentioning **Kubernetes**. **Dynamic routing** reads each alert's `service` field: the memory leak, node `MemoryPressure` and claims queue backlog go to `claims-reconciliation`; the evicted pods go to `scheduling`.
- Each service's EO rule matches *Kubernetes*, raises it to **critical**, and adds an annotation that points at the change and the runbook. No priority yet: that's a human call (beat 5).
- **Cross-service Intelligent Alert Grouping** (one grouping setting covering both services, comparing the alert `source`, which is the node) folds alerts from `claims-reconciliation` **and** `scheduling` into **one incident**.
- Both services light up. `claims-reconciliation` gets the memory leak, node pressure and queue backlog. `scheduling` gets latency with readiness failing, then the eviction, then 503s at the ingress.
- **Say:** "One orchestration, no routing table: each alert says which service it belongs to. Seven alerts on two services, one incident, because PagerDuty saw they share a node. Scheduling looks like the broken one, and it even shipped a change today, but it's a victim."
- *Depends on:* global orchestration with dynamic routing, service EO rules, cross-service alert grouping (intelligent grouping learns, so the first run on a new account may not group; `alert_grouping_type = "content_based"` makes it deterministic) ([02](../instructions/02-terraform.md)).

### 3. Business impact is visible immediately

- **Show:** the service graph or Business Services view. `patient-portal` shows **impacted** through its dependencies on `claims-reconciliation` and `scheduling`. The internal status page shows it too.
- **Say:** "We're not reading node names any more. This is the patient portal being down."
- *Depends on:* business services and dependencies (Terraform), internal status page ([06](../instructions/06-paige-runbook-status-page.md)).

### 4. Paige is first responder

- The escalation policy pages **Paige at level 1**. Two conditional workflows fire on creation:
  - **Paige Virtual Responder - Dedicated Slack Channel** creates the incident channel. Paige posts findings and next steps there.
  - **Kubernetes diagnostics** lists the claims-reconciliation pods through the Agent Connector and sends them to Paige as an incident note.
- **Paige's finding** comes from the change event, the alert custom details and the runbook: memory grew linearly from the `v2.14.0` rollout, there is no memory limit, the evictions are collateral, and the root cause is the `x12-claims-parser` upgrade. She also rules out the scheduling change: it was config-only, healthy for 3 hours, and scheduling's own memory is normal. Recommended fix: restart the pod and apply a memory limit.
- **Say:** "Before a human has opened a laptop, Paige has a channel, the live pod list, and a root cause tied to a specific change."
- *Depends on:* Slack and Paige scopes, the Agent Connector, the runner, the runbook upload ([03](../instructions/03-kubernetes-and-runner.md), [04](../instructions/04-integrations.md)).

### 5. A person decides, the platform mobilises

- Paige recommends **Major Incident** (customer impact). You approve.
- The workflow sets **P1**, adds responders and stakeholders, opens a **Zoom** bridge, and adds **Scribe** to capture decisions.
- **Say:** "Paige recommends, a human approves. That's the control point."

### 6. Fix it from the incident

- Paige recommends **Kubernetes Remediation**. You approve.
- The workflow deletes the leaking pod and waits. Then it runs a **pod command** that sets `limits.memory=512Mi` on the Deployment. In the ad-hoc variant, it creates a replacement pod instead.
- **Show:** `kubectl -n healthcare get pods -w` (optional second screen). The new pod comes up, and `patient-portal` and `scheduling` reschedule.
- **Say:** "The approved runbook fix, executed by the runner inside the cluster, logged on the incident."

### 7. Close the loop

- **Resolve demo alerts** in `./trigger`. The alerts on both services resolve, `patient-portal` goes back to operational, and the status page clears.
- **Show:** the incident timeline. It contains the change event, the EO annotation, Paige's notes, the workflow runs, and the Scribe summary.
- **Say:** "Detection to fix in minutes, with the record written for us. And the follow-up is clear: pin the library and enforce memory limits."

## One-line versions

- **Executive:** A routine library upgrade quietly took down patient-facing services. PagerDuty found the change, mobilised the right people, and fixed it in-cluster, with a human approving every step.
- **Technical:** Unbounded memory growth from a dependency bump caused node MemoryPressure and neighbour evictions. EO, grouping and dependencies give the business view. Paige ties it to the change event, and the runner applies the runbook fix.
