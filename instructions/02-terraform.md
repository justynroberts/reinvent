# 02: Terraform (services, dependencies, grouping, EO, workflow shells)

Everything in `terraform/` is plain PagerDuty provider config (`PagerDuty/pagerduty ~> 3.26`), with local state.

## What it creates

| File | Resources |
|---|---|
| `main.tf` | Escalation policy (you as responder), `claims-reconciliation` and `scheduling` technical services, a change-event integration on each technical service, cross-service intelligent alert grouping on both (compares `source`, the node), the `patient-portal` business service and its two dependencies. `moved` blocks keep an earlier claims-reconciliation in place instead of recreating it. |
| `orchestration.tf` | **Reinvent Patient** global orchestration. Its one router rule is dynamic routing: `event.custom_details.service` is looked up as a service name. Unmatched alerts go to claims-reconciliation. Each technical service also has a service orchestration: rule 1 suppresses `info`, and rule 2 matches *Kubernetes* and sets critical, an annotation and `orchestration_rule` (no priority: Major Incident sets P1). |
| `workflows.tf` | Four incident workflows with the **exact** names, their descriptions, and triggers scoped to `claims-reconciliation`: two conditional (`incident.urgency matches 'high'`) and two manual |
| `outputs.tf` | `routing_key` (orchestration, for alerts) and `change_routing_keys` (one change integration per service, as a name→key map), both sensitive; service, business service and workflow IDs and URLs |

## Run it

```bash
./trigger      # menu option 2 previews (read-only), option 3 creates
# or directly: ./trigger plan, ./trigger create
```

Before Terraform runs for the first time in a session, `plan`, `create` and `destroy` ask three questions. If a value is already saved, it's shown in `[brackets]`: press Enter to keep it or type a new one.

1. **Region:** US or EU. EU if your PagerDuty URL is `*.eu.pagerduty.com`.
2. **API key:** hidden input, checked read-only against PagerDuty. If it's rejected, you're offered the other region before re-entering.
3. **Your email:** looked up on the account, so a typo or wrong account is caught here, not halfway through `apply`.

You then confirm a summary, and the values are saved to `.env` (mode 600, gitignored). The email is passed to Terraform as `-var responder_email=…`, which overrides `terraform.tfvars`. `terraform.tfvars` is now only needed for the optional settings in `terraform.tfvars.example`.

Without a terminal (CI, piped input), nothing is asked: saved values are checked, and the command stops with a clear message if one is missing.

`./trigger create` runs `terraform init` then `terraform apply` and leaves the confirmation prompt in place.

## Things that will bite

- **Names collide.** If the sandbox already has a service or workflow with one of these names, apply fails or you end up with a duplicate. Delete the old one or import it with `terraform import`.
- **Paige on the EP.** After you add Paige at level 1 in the UI, Terraform ignores `rule` changes on the escalation policy, so a re-apply won't remove her. The trade-off is that changing the human level in Terraform has no effect; edit it in the UI.
- **Workflow steps.** Same pattern: Terraform ignores `step`. If you'd rather build all four workflows from UI templates, set `manage_incident_workflows = false` *before* the first apply. Otherwise you'll get name clashes.
- **Intelligent grouping on a fresh service** may not merge the four alerts on the first run. For a guaranteed single incident, use `alert_grouping_type = "content_based"`, which groups on `source` (every payload sets the node name there).
- **Destroy** removes everything Terraform created, including the workflow shells and any steps you added to them. Screenshot or export your steps first. `./trigger destroy` (menu option 9) resolves the scenario's alerts before it destroys.
