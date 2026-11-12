# 06: Paige on the escalation policy, runbook upload, status page

## Paige at level 1

Requires the *Paige on Escalation Policies* EA flag.

1. People → Escalation Policies → **claims-reconciliation escalation** (created by Terraform).
2. Insert a new **level 1** with **Paige** as the target, and turn on **Trigger SRE Agent**.
3. Leave yourself as the human responder at the next level. Terraform set the delay to 10 min. Change it in the UI if you need to, because Terraform ignores rule edits on this policy.

## Runbook

1. The source is [`runbook/claims-reconciliation.md`](../runbook/claims-reconciliation.md).
2. Upload it as the runbook for **claims-reconciliation**, in the service's SRE Agent / knowledge area. The exact location depends on the EA build.
3. The fix Paige should cite: **restart the pod and apply a memory limit to the claims-reconciliation deployment**. The runbook says this in its *Remediate* section.
4. Check by asking Paige in Slack: *"What's the runbook fix for claims-reconciliation memory growth?"*

## Internal status page

The provider has no status page resources, so this is UI only.

1. Status Pages → create an **internal** status page (for example "Patient Services").
2. Add the business services **patient-portal** and **scheduling**.
3. During the demo, they show impacted through their dependency on `claims-reconciliation`.
