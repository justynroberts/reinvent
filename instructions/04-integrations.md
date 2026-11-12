# 04: Integrations and connectors

All OAuth or UI-only. None of it can be scripted.

## Slack and Paige

- [ ] Integrations → Extensions → **Slack** connected to the demo workspace.
- [ ] **PagerDuty AI** (Paige) connected to Slack.
- [ ] **Reauthorize** the Slack integration after Paige is enabled, so it grants Paige's additional scopes. If you skip this, the dedicated-channel workflow creates a channel but Paige can't post in it, and the demo silently loses its best beat.
- [ ] Invite yourself to a test channel and check that Paige answers an @-mention.

## Video bridge

- [ ] **Zoom** (or Google Meet) connected under Integrations → Extensions. The Major Incident workflow's *Create a Zoom Meeting* step needs it. If you use Meet, swap that step for the Meet equivalent.

## Kubernetes Agent Connector

- [ ] Create a **Kubernetes Agent Connector** that uses the runner from 03, with **all tools enabled**.
- [ ] Check that it can list pods in `healthcare`. The *Kubernetes diagnostics* workflow's *List Objects* step goes through this connector.

## Change events

Nothing to configure. Alerts go to the Reinvent Patient orchestration's routing key, but change events go to claims-reconciliation's own integration key at `/v2/change/enqueue`. `trigger run` sends one first, with its timestamp backdated 47 minutes (`vars.deployed_ago`), so the incident's *Recent changes* shows the `v2.14.0` rollout.

Check: after a run, Service → `claims-reconciliation` → **Recent Changes** lists "Deployed claims-reconciliation v2.14.0 …".

If you'd rather use Postman or cURL, as in the original brief:

```bash
./trigger run -dry-run          # prints every rendered body; paste one into Postman
```
