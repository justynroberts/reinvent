# claims-reconciliation memory leak: ready-to-send events

Generated from `payloads/scenario.json` by `./trigger export`. Plain JSON for the PagerDuty Events API v2: send them with `./send.sh`, Postman or any HTTP client, then close the alerts with `./resolve.sh`.

| # | File | Endpoint | Routed to |
|---|---|---|---|
| 1 | `01-change-event-scheduling-v3-8-2-config-only-red-herring.json` | `/v2/change/enqueue` | `scheduling` (its change integration key) |
| 2 | `02-change-event-v2-14-0-rollout-library-upgrade.json` | `/v2/change/enqueue` | `claims-reconciliation` (its change integration key) |
| 3 | `03-noise-probe-latency-info-suppressed-by-eo.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |
| 4 | `04-claims-reconciliation-memory-leak-worker-hung.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |
| 5 | `05-splunk-claims-x12-parse-error-spike.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |
| 6 | `06-scheduling-latency-readiness-failing.json` | `/v2/enqueue` | `scheduling` (Reinvent Patient orchestration, dynamic routing) |
| 7 | `07-rds-scheduling-db-read-latency.json` | `/v2/enqueue` | `scheduling` (Reinvent Patient orchestration, dynamic routing) |
| 8 | `08-node-memorypressure-kubelet-evicting.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |
| 9 | `09-scheduling-pods-evicted.json` | `/v2/enqueue` | `scheduling` (Reinvent Patient orchestration, dynamic routing) |
| 10 | `10-scheduling-503s-at-the-ingress.json` | `/v2/enqueue` | `scheduling` (Reinvent Patient orchestration, dynamic routing) |
| 11 | `11-synthetics-book-an-appointment-journey-failing.json` | `/v2/enqueue` | `scheduling` (Reinvent Patient orchestration, dynamic routing) |
| 12 | `12-claims-submission-queue-backing-up.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |
| 13 | `13-clearinghouse-999-acknowledgements-overdue.json` | `/v2/enqueue` | `claims-reconciliation` (Reinvent Patient orchestration, dynamic routing) |

## Notes

- **Routing keys.** Alerts use the global orchestration key, and change events use each service's own change key. In the committed copy these are placeholders (`YOUR_ORCHESTRATION_ROUTING_KEY`, `YOUR_<SERVICE>_CHANGE_KEY`). Export with `./trigger export -keys` (menu: Export payloads) to get a copy with your real keys.
- **Timestamps.** Alerts have none, so PagerDuty uses the time it receives them. Change events keep a timestamp backdated from the export time; re-export for a fresh one.
- **Resolve** files are the full alert with `event_action: resolve`, so dynamic routing takes them to the same service.
- Every alert carries `custom_details.service`, which the orchestration routes on and uses to add `custom_details.runbook.github`.
