# 01: Platform prerequisites

All of this is manual. Check it before any other setup step.

## Account

- [ ] **Subdomain is on the PD Reliability Platform plan.** Paige, Intelligent Alert Grouping, Event Orchestration, business services and status pages all depend on it.
- [ ] **Priorities are enabled** (Account Settings → Incident Priority). The EO rule sets P1. If priorities are off, set `enable_priority = false` in `terraform.tfvars`.
- [ ] **EA flags enabled on the subdomain** (request through your PagerDuty contact; these are per subdomain, not per user):
  - [ ] Paige on Escalation Policies
  - [ ] PD Automation Runner
- [ ] **You exist as a user** with a working notification method. `./trigger` asks for your email and checks the user exists.

## The Core Demo Flow runs per sandbox

Each presenter configures the full flow in their own sandbox. Don't share services, because workflows, connectors and the runner are all tied to one account.

## Navigation

The UI has both a **left nav** and a **top nav**, and the demo moves between them. Before presenting, practise:

- Incidents → the console
- Services → Service Directory → `claims-reconciliation`
- Services → Business Services (impact view)
- Automation → Incident Workflows → Automation Connectors → Self-hosted Runners
- Status Pages

Where an item lives depends on the EA build. Walk the path once in your own sandbox rather than trusting a screenshot.

## Console view

- [ ] In the incidents console, add the **Paige** column and the **Alerts** field.
- [ ] Save it as a named view, and make it your default so it's what loads when you open Incidents on stage.
