# Setup instructions

Work through these in order. The whole setup takes about 90 minutes the first time and about 10 to rebuild.

| # | Doc | What |
|---|---|---|
| 01 | [01-platform.md](01-platform.md) | Plan, EA flags, nav, console view |
| 02 | [02-terraform.md](02-terraform.md) | Services, business services, dependencies, EP, grouping, EO, workflow shells |
| 03 | [03-kubernetes-and-runner.md](03-kubernetes-and-runner.md) | kind cluster, demo workloads, Automation Runner, toolbox RBAC |
| 04 | [04-integrations.md](04-integrations.md) | Slack and Paige scopes, Zoom/Meet, Kubernetes Agent Connector, change events |
| 05 | [05-incident-workflows.md](05-incident-workflows.md) | Steps for the four workflows |
| 06 | [06-paige-runbook-status-page.md](06-paige-runbook-status-page.md) | Paige on the EP, runbook upload, internal status page |
| 07 | [07-preflight.md](07-preflight.md) | Dry-run checklist, reset between demos, enablement sessions |

## What is automated, and what isn't

🟢 automated · 🟡 partly automated, finish in the UI · 🔴 manual (no API or provider support, or an EA feature)

| Item | Status | How / why not |
|---|---|---|
| `claims-reconciliation`, `scheduling` technical services | 🟢 | Terraform `pagerduty_service` |
| `patient-portal` business service | 🟢 | Terraform `pagerduty_business_service` |
| Service dependencies (patient-portal → both technical services) | 🟢 | Terraform `pagerduty_service_dependency` |
| **Reinvent Patient** global orchestration, dynamic routing on `custom_details.service` | 🟢 | Terraform `pagerduty_event_orchestration` + `_router`. Alerts use its routing key (`terraform output routing_key`). |
| Runbook link on every alert (`custom_details.runbook.github` → `runbook/<service>.md` on GitHub) | 🟢 | Terraform `pagerduty_event_orchestration_global` rule |
| Change events on both technical services | 🟢 | One integration per service (`terraform output change_routing_keys`). The app picks the key by the change event's `custom_details.service`. |
| Alert payloads (mention Kubernetes, root-cause context) | 🟢 | `payloads/`, rendered and sent by `trigger run` |
| Cross-service Intelligent Alert Grouping (both services, compares `source` = node) | 🟢 | Terraform `pagerduty_alert_grouping_setting` with both services and `iag_fields = ["source"]`. IAG learns, so a fresh account may not group the first run; `alert_grouping_type = "content_based"` makes it deterministic. |
| Service orchestration rule on both technical services (Kubernetes → critical + annotation; info suppressed; no priority) | 🟢 | Terraform `pagerduty_event_orchestration_service` |
| Escalation policy, human responder | 🟢 | Terraform |
| **Paige at level 1** with Trigger SRE Agent on | 🔴 | EA feature, not in the provider. Added in the UI; Terraform ignores rule changes so it survives re-apply. |
| Four incident workflows: names, descriptions, triggers | 🟢 | Terraform `pagerduty_incident_workflow` + `_trigger` |
| Major Incident and Paige workflow **steps** | 🟢 | Terraform, with action IDs checked on the account. Zoom must be connected (04); the Slack channel step needs `slack_workspace_id`. |
| Kubernetes workflow **steps** | 🔴 | They bind to the Agent Connector and runner, which the provider can't look up. Added in the UI; Terraform ignores `step`. See 05. |
| Demo Kubernetes workloads (leaky pod, neighbours, toolbox) | 🟢 | `k8s/*.yaml`, `./trigger cluster up` |
| kind cluster | 🟢 | `./trigger cluster up` (needs Docker + `kind`) |
| PD Automation Runner: register, credentials | 🔴 | Registration issues `RUNNER_ID` / `RUNNER_SECRET` in the UI |
| PD Automation Runner: deploy and RBAC | 🟡 | Manifest from the runner KB / UA Runner Kind script. This repo adds only the narrow toolbox RBAC. |
| Kubernetes Agent Connector (all tools) | 🔴 | UI only |
| Slack, PagerDuty AI in Slack, Paige scopes reauth | 🔴 | OAuth flows |
| Zoom / Google Meet | 🔴 | OAuth flow |
| Runbook content (troubleshooting only, no escalation) | 🟢 | `runbook/claims-reconciliation.md`, `runbook/scheduling.md` |
| Runbook upload to the service | 🔴 | UI only |
| Internal status page showing the patient-portal business service | 🔴 | The provider has no status page resources; the REST API doesn't create pages |
| Console view with Paige column + Alerts field | 🔴 | Per-user UI setting |
| EA flags (Paige on EPs, PD Automation Runner) | 🔴 | Requested through PagerDuty, per subdomain |

## Credentials you need

All of these can live in `.env` at the repo root (see `.env.example`). The trigger app loads it, and variables set in your shell take precedence.

| Variable | Used by | Where from |
|---|---|---|
| `PAGERDUTY_SERVICE_REGION` | Terraform and the trigger app | `us` or `eu`. Asked by `./trigger` before the first Terraform step. |
| `PAGERDUTY_TOKEN` | Terraform (`./trigger plan` / `create` / `destroy`). Asked for (hidden input), or confirmed if already saved, and checked read-only against `GET /abilities`. | User Icon → My Profile → User Settings → API access, or Integrations → API Access Keys (account key, read/write) |
| `PD_RESPONDER_EMAIL` | Terraform's `responder_email` (passed as `-var`) | Your PagerDuty login email. Asked for, or confirmed if already saved, and looked up on the account. |
| `PD_ROUTING_KEY` / `PD_CHANGE_ROUTING_KEY` | Trigger app (optional) | Override the alert / change-event keys from the Terraform outputs |
| Runner ID / secret / token | Runner pod | Registering the runner (03) |
