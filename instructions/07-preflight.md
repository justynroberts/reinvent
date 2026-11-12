# 07: Pre-flight, reset, enablement

## Day-before dry run

- [ ] `make test` passes (the payloads render)
- [ ] `./trigger plan` (menu option 2) shows no changes (nothing has drifted)
- [ ] Runner is **Healthy**; `kubectl -n healthcare get pods` shows all four pods Running
- [ ] `./trigger run` (menu option 5) produces **one** P1 incident on `claims-reconciliation` containing four alerts. The info noise event is suppressed (check the service's alert log).
- [ ] Recent Changes shows the `v2.14.0` deploy
- [ ] EO annotation is on the incident
- [ ] `patient-portal` and `scheduling` show impacted (Business Services, internal status page)
- [ ] Paige is paged at level 1 and the dedicated Slack channel appears with her findings
- [ ] Kubernetes diagnostics note ("Send Pods to Paige") lists the claims-reconciliation pod
- [ ] Paige's root cause names the `x12-claims-parser` upgrade, and her recommended fix matches the runbook
- [ ] Major Incident (manual): responders, stakeholders, Zoom link, Scribe joins
- [ ] Kubernetes Remediation (manual): pod deleted and recreated, memory limit applied
- [ ] Console view shows the Paige column + Alerts field
- [ ] `./trigger resolve` (menu option 7), then the status page clears

## Reset between demos

```bash
./trigger resolve          # menu 7: resolve all scenario alerts
./trigger cluster reset    # menu 8: put the leaky Deployment back (removes the memory limit)
# Archive or delete the Slack channel Paige created
```

The alert dedup keys are stable, so running the scenario again while an incident is still open adds to the same alerts instead of creating new ones. Always resolve first.

## Tear down

```bash
./trigger destroy          # menu 9: resolves, then terraform destroy (prompts)
./trigger cluster down     # menu 10
```

## Enablement sessions to check

- [ ] Paige and Automation
- [ ] MCP
- [ ] SRE Workbench (PD Labs)
