# 07: Pre-flight, reset, enablement

## Day-before dry run

- [ ] `make test` passes (the payloads render)
- [ ] SETUP → 3 *Preview PagerDuty changes* says nothing to do (nothing has drifted)
- [ ] Runner is **Healthy**; `kubectl -n healthcare get pods` shows all four pods Running
- [ ] DEMO → 1 *RUN THE DEMO* opens a fresh incident on `claims-reconciliation` that groups the Kubernetes alerts from both services (no priority until Major Incident sets P1). The info noise event is suppressed (check the service's alert log).
- [ ] Recent Changes shows the `v2.14.0` deploy
- [ ] EO annotation is on the incident
- [ ] `patient-portal` and `scheduling` show impacted (Business Services, internal status page)
- [ ] Paige is paged at level 1 and the dedicated Slack channel appears with her findings
- [ ] Kubernetes diagnostics note ("Send Pods to Paige") lists the claims-reconciliation pod
- [ ] Paige's root cause names the `x12-claims-parser` upgrade, and her recommended fix matches the runbook
- [ ] Major Incident (manual): responders, stakeholders, Zoom link, Scribe joins
- [ ] Kubernetes Remediation (manual): pod deleted and recreated, memory limit applied
- [ ] Console view shows the Paige column + Alerts field
- [ ] DEMO → 2 *Resolve the demo*, then the status page clears

## Reset between demos

```bash
./trigger resolve          # DEMO → 2: resolve every open demo run
kubectl --context kind-reinvent -n healthcare delete deployment/claims-reconciliation pod/claims-reconciliation --ignore-not-found && kubectl --context kind-reinvent apply -f k8s/healthcare.yaml   # only if you use the cluster
# Archive or delete the Slack channel Paige created
```

The alert dedup keys are stable, so running the scenario again while an incident is still open adds to the same alerts instead of creating new ones. Always resolve first.

## Tear down

```bash
./trigger destroy          # SETUP → 6: resolves, then deletes (shows the list, asks)
kind delete cluster --name reinvent   # only if you created it
```

## Enablement sessions to check

- [ ] Paige and Automation
- [ ] MCP
- [ ] SRE Workbench (PD Labs)
