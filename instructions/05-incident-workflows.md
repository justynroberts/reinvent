# 05: Incident workflows

Terraform has already created all four workflows with the **exact names**, descriptions and triggers. **Major Incident** and **Paige Virtual Responder** come complete, steps included. Only the two Kubernetes workflows need steps added under Automation → Incident Workflows, because those steps go through the Agent Connector and runner. Publish each one when you're done.

If you set `manage_incident_workflows = false`, create each workflow from its template instead, and set the name, trigger and service scope by hand to match the table.

| Workflow (exact name) | Trigger | Created by Terraform |
|---|---|---|
| Paige Virtual Responder - Dedicated Slack Channel | Conditional on creation (`incident.urgency matches 'high'`), both services | 🟢 complete, steps included |
| Kubernetes diagnostics | Conditional on creation, claims-reconciliation | 🟢 shell + trigger, 🔴 steps |
| Major Incident | Manual, both services | 🟢 complete, steps included |
| Kubernetes Remediation | Manual, claims-reconciliation | 🟢 shell + trigger, 🔴 steps |

Terraform owns the steps of the two complete workflows, so make changes in `terraform/workflows.tf`, not the UI: the next apply reverts UI edits. The Kubernetes steps are 🔴 because they bind to the Agent Connector and runner, which the provider can't look up. Terraform ignores their steps.

---

## Paige Virtual Responder - Dedicated Slack Channel

- **Description:** Creates a dedicated Slack channel for the incident and brings in Paige to investigate.
- **Steps (Terraform):** **Add Paige** posts her investigation in PagerDuty and as an incident note. If `slack_workspace_id` is set in `terraform.tfvars` (your Slack workspace ID, which starts with `T`), a **Create Incident Dedicated Channel** step comes first and Paige also posts in that channel.
- **Check:** Paige's findings appear on the incident within about a minute. With Slack on, the channel appears too. If it doesn't, see the Slack reauth note in 04.

## Kubernetes diagnostics

Reference: the orbitpay example linked in [story/brief.md](../story/brief.md).

1. **List Objects**, renamed **List claims-reconciliation pods**
   - Connector: the Kubernetes Agent Connector / runner from 03
   - Kind: Pod · Namespace: `healthcare` · Label selector: `app=claims-reconciliation`
2. **Add Note to an Incident**, renamed **Send Pods to Paige**
   - Note body: the output of step 1. Insert it with the workflow's variable picker; field names differ by action version.

## Major Incident

- **Description:** For P1 incidents with customer impact. Adds responders and stakeholders, opens a Zoom bridge and brings in Scribe to capture decisions.
- **Trigger:** manual. It runs only when Paige recommends it and a person approves.
- **Steps (Terraform), in order:**
  1. **Set priority P1** (skipped if `enable_priority = false`)
  2. **Add responders**: you, as a stand-in for a Patient Experience team
  3. **Add stakeholders**: you, plus `team_id` if set
  4. **Create a Zoom meeting**: needs the Zoom integration connected (04). Set `major_incident_conference = "none"` to drop this step and Scribe.
  5. **Add Scribe**: joins the Zoom call

## Kubernetes Remediation

Reference: the orbitpay example linked in [story/brief.md](../story/brief.md). Build the variant that matches how claims-reconciliation runs in your cluster. `./trigger cluster up` runs it as a **Deployment**.

### Variant A: Pod Created through Deployment (default)

1. **Delete Pod**: namespace `healthcare`, label selector `app=claims-reconciliation`. The Deployment recreates it.
2. **Delay**: 15–20 s
3. **Pod Command** (update memory limit): pod `claims-toolbox`, namespace `healthcare`, container `worker`, command:
   ```sh
   kubectl -n healthcare set resources deployment/claims-reconciliation -c app --requests=memory=256Mi --limits=memory=512Mi
   ```

### Variant B: Pod Created Adhoc

1. **Delete Pod**: namespace `healthcare`, name `claims-reconciliation`
2. **Delay**: 15 s
3. **Create Pod**: use the spec in `k8s/claims-reconciliation-adhoc-pod.yaml` (same app, with a memory limit). This mimics Kubernetes spinning up a new pod from a Deployment.

**Check (A):**

```bash
kubectl -n healthcare get deploy claims-reconciliation -o jsonpath='{.spec.template.spec.containers[0].resources}'
# {"limits":{"memory":"512Mi"},"requests":{"memory":"256Mi"}}
```

Patching resources triggers a rollout, so the pod restarts a second time. That's expected.

---

## Not verified here

Action names, field labels and the *Surface → Slack* path all come from the brief and the templates, and EA builds change them. Run each workflow once by hand against a test incident before the demo ([07](07-preflight.md)).
