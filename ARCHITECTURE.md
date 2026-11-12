# Architecture

How this repo fits together: commands, the non-obvious decisions, and the things that bite. Read it before changing anything.

## What this is

Demo kit for a PagerDuty story: a memory leak in `claims-reconciliation` (from a library upgrade) evicts `scheduling` on a shared Kubernetes node, which takes down the `patient-portal` business service. Paige diagnoses it, and incident workflows remediate it. `story/README.md` is the narrative, and every technical choice here serves it. Read it before changing payloads or names.

## Commands

**The user's entry point is `./trigger`**, a root shell script, and they want to run nothing else. It rebuilds `bin/trigger` from `app/` when Go sources are newer, then execs it. With no args on a TTY it opens a full-screen TUI (`app/menu.go`): REINVENT ASCII banner, status panel (settings, PagerDuty state from `terraform.tfstate`, kind cluster), and a menu driven by arrow keys, j/k, digits 0-9 or Enter, with ★ on the suggested next step. It's stdlib-only: raw mode goes through `stty`, the terminal is restored on exit and on Ctrl-C, and `NO_COLOR` is honoured. Status shells out to kind/docker, so it refreshes only after an action, not on every keypress. Any new user-facing task belongs in that menu, not in a new script or Make target. The `Makefile` is an optional thin wrapper around `./trigger`.

```bash
make validate          # offline: terraform validate + go vet + go test (renders every payload)
make test              # go vet + go test only
./trigger run -dry-run -no-wait                     # print every rendered event, send nothing
cd app && go test -run TestPayloadsRender ./...     # one test
./trigger plan         # read-only terraform plan (runs the setup questions first)
```

`./trigger create`, `run`, `resolve`, `destroy` and `cluster up|reset|down` change a live PagerDuty account or cluster. **Don't run them unless the user asks in that message.** `validate`, `test`, `-dry-run` and `plan` are safe. The app finds the repo root by walking up to `payloads/scenario.json` (override with `-root`, which goes after the subcommand).

## Distribution

Most users don't have a checkout. They install with `install.sh` (`curl … | sh`), which unpacks the release zip into `~/Applications/Reinvent.app` and opens it. Files fetched by curl aren't quarantined, which is why an unsigned app works this way. **This Mac has no Developer ID or notary profile**, so releases are ad-hoc signed. `scripts/release.sh` signs and notarises automatically if both ever appear.

- **One binary, two modes.** If walking up from the working directory finds `payloads/scenario.json`, the app uses the checkout (dev mode). Otherwise it uses the workspace `~/Library/Application Support/Reinvent`: `.env`, Terraform state and `tools/` live there.
- **Embedded files:** `assets.go` (root package, which is why `go.mod` is at the repo root) embeds `payloads`, `k8s`, `runbook` and the Terraform files. On a version change they're re-extracted into the workspace (`app/workspace.go`), always for `dev` builds. `.env`, state and tools are never touched. A new file the app needs at run time must be added to the `//go:embed` line.
- **Tools** (`app/tools.go`): Terraform, kind and kubectl are pinned and downloaded on first use from the publishers, checked against their SHA-256 files, and stored in `<workspace>/tools`. PATH copies win. Docker can't be downloaded; `dockerOutsidePath` finds Docker Desktop off-PATH and puts it on PATH for kind.
- **Finder launch** (`app/launch.go`): with no TTY, running from inside `.app`, it reopens itself with `open -a Terminal`.
- **Releases:** `scripts/release.sh <ver>` builds a universal binary (`-X main.version`), creates the bundle and zip, then verifies a fresh copy from an empty `HOME` outside the checkout. `--publish` tags and runs `gh release create`; only use it when the user asks in that message. Releases are cut locally, never in CI. `githubRepo` in `app/workspace.go` and `REPO` in `install.sh` must match.

## Setup must be idiot-proof (first rule)

Users are often non-technical. Every change to setup is judged against that:
- **Before Terraform runs** (`app/preflight.go`): priorities off → `-var=enable_priority=false` with a note. Objects with our names that aren't in local state → stop and list them with links (`ownedObjects`; `TestOwnedObjectsMatchTerraform` keeps it in step with `terraform/*.tf`).
- **No raw Terraform:** `planAndApply` runs init/plan quietly, prints "Will create / update / DELETE" in plain words, asks y/N (the default is No when anything is deleted), then applies that exact plan file. `tfFailed` maps 401/403/name-taken/network/lock errors to a one-line fix and shows the tail of the output.
- Docker can be Docker Desktop, Rancher Desktop (`~/.rd/bin`) or OrbStack (`~/.orbstack/bin`).

## How the pieces connect

- **Setup (`app/setup.go`):** the app loads `<root>/.env`, and the real environment wins. `plan`, `create` and `destroy` call `ensureSetup`. On a TTY it asks for region (`PAGERDUTY_SERVICE_REGION`, us/eu), API key (`PAGERDUTY_TOKEN`, checked with read-only `GET /abilities`) and responder email (`PD_RESPONDER_EMAIL`, checked with `GET /users?query=`). Saved values are shown for keep-or-change, then a final summary is confirmed and cached for the session, so the menu asks only once. It fetches `terraform` if it's missing. Off a TTY nothing is prompted, and saved values are validated or it fails. Everything is saved to `.env` at mode 600. Terraform inherits the env (region is unset for US) and gets `-var responder_email=…`, which beats `terraform.tfvars`. `.env` is gitignored; never read its contents into output.
- **Topology:** `patient-portal` is the only business service. It depends on two technical services, `claims-reconciliation` (the culprit) and `scheduling` (evicted neighbour). Both share one EP, alert grouping setting and service-orchestration rules (`for_each` over `local.technical_services`).
- **Routing:** alerts go to the **Reinvent Patient** global orchestration (`output routing_key`). Its only rule is dynamic routing on `event.custom_details.service`, looked up by **exact service name**. Every alert payload must carry `custom_details.service` = `{{.Vars.claims_service}}` or `{{.Vars.scheduling_service}}`. `TestServiceNamesMatchTerraform` checks those vars against the `variables.tf` defaults. The catch-all goes to claims-reconciliation. Change events skip the orchestration: each technical service has its own change integration (`output change_routing_keys`, a name→key map read with `terraform output -json`), and the sender picks the key by the change event's `custom_details.service`. Overrides: `PD_ROUTING_KEY`, and `PD_CHANGE_ROUTING_KEY` (used for every service).
- **Upgrades:** `requiredOutputs` in `app/menu.go` lists the Terraform outputs this version reads. A state with resources but missing one of them counts as "outdated". The status panel then shows "needs update", ★ moves to option 3, and `run` fails with a plain-English message before anything is sent. When you add an output the app reads, add it there.
- **Runbook link:** a global-orchestration rule (`pagerduty_event_orchestration_global`, which runs before the router) reads `custom_details.service` into a variable and sets `event.custom_details.runbook.github` = `<runbook_base_url>/<service>.md`. Every service therefore needs `runbook/<service name>.md` in this repo, on `main`. `TestRunbooksExist` checks this, and checks that runbooks never mention escalation: they hold troubleshooting steps only (the user's rule).
- **Resolve** sends the full rendered alert body with `event_action: resolve`, not just the `dedup_key`, so dynamic routing delivers it to the same service. Keep this.
- **payloads/ is data, not code.** `scenario.json` holds `vars` + ordered `steps`, and each step points to a Go `text/template` JSON file (`{{.Vars.x}}`, `{{now}}`, `{{ago "47m"}}`, `missingkey=error`). The routing key is injected at send time. `app/scenario_test.go` renders the real files, so a broken payload edit fails `go test`.
- **`resolve` and `destroy`** resolve by each alert payload's `dedup_key`. Keep the keys stable and non-empty.
- **Payloads ↔ service EO rule:** `terraform/orchestration.tf` matches `(?i)kubernetes` in `event.summary` or `custom_details.platform == kubernetes`, and suppresses `severity == info`. The scenario mixes Kubernetes alerts with other tools' alerts (CloudWatch RDS, Splunk, Datadog Synthetics, the clearinghouse), which keep their own severity. `TestPayloadsRender` checks that a summary mentions Kubernetes **if and only if** `platform == kubernetes`.
- **Template gotcha:** payloads are JSON, so a string argument inside `{{ }}` takes backticks, not escaped quotes: `{{ago `+"`44m`"+`}}`.
- **`payloads/rendered/`** is generated, plain JSON for use outside this app (curl/Postman), plus `send.sh`, `resolve.sh` and `resolve/`. Placeholder keys only. `TestRenderedUpToDate` fails when it's stale; regenerate with `./trigger export`. Menu **e** (`export -keys -open`) writes a copy with the real keys, mode 600, to `~/Desktop/Reinvent payloads`. Alert timestamps are dropped there; change-event timestamps are backdated from export time.
- **Payloads ↔ cluster:** `vars.node`, `vars.namespace` and `vars.deployment` match `k8s/kind-config.yaml` (node `reinvent-control-plane`) and `k8s/healthcare.yaml` (namespace `healthcare`, label `app=claims-reconciliation`, container `app`). Change them together.
- **Alert grouping is cross-service.** One `pagerduty_alert_grouping_setting` covers both technical services, so the scheduling alert can join the claims incident. It's `intelligent` with `iag_fields = ["source"]`: every alert sets `source` to the node name, the signal they share. Comparing on `summary` (the old default) grouped nothing. IAG learns, so a fresh account may not group the first run; `alert_grouping_type = "content_based"` (also on `source`) is the deterministic fallback.
- **No automatic priority.** The service EO rule sets critical and an annotation only. P1 is set by the Major Incident workflow when a person declares it (the user's call: incidents shouldn't arrive as P1).

## Deliberate Terraform choices

- `lifecycle.ignore_changes = [rule]` on the escalation policy, because Paige is added at level 1 in the UI (EA, no provider support).
- Incident workflows are split. `pagerduty_incident_workflow.managed` (Major Incident, Paige Virtual Responder) has its **steps in Terraform**; the action IDs were checked against `GET /incident_workflows/actions` on the account, and the input formats (`[{"id","type"}]` JSON, priority = ID) were copied from existing workflows. `.manual` (the two Kubernetes workflows) keeps `ignore_changes = [step]`, because its steps bind to the Agent Connector/runner and are added in the UI. Before adding a new action ID, confirm it exists on the account. Inputs you don't set get API defaults.
- Paige can't be put on an escalation policy through the API or provider (target types are user/schedule only). The **Add Paige** workflow action (`pagerduty-advance:auto-triage`) in the conditional Paige workflow does the first-responder job; EP placement stays a UI step.
- Workflow and service names are **exact strings from the brief**. Don't rename them. Exception, by the user's decision: `patient-portal` is the only business service and `scheduling` is a technical service. `story/brief.md` still says two business services; leave the brief as the original.
- No status page resources exist in the provider; that's manual, by design.

## Kubernetes

`./trigger cluster` (`app/cluster.go`) always passes `--context kind-reinvent` (override with `-context`), so it never acts on whatever cluster is current. `cluster down` refuses anything but the kind cluster. Missing tools and a stopped Docker fail with an install or start hint, not a stack trace.

The runner (from the PD Automation Runner KB, not in this repo) keeps its read-mostly ClusterRole. The memory-limit patch runs via `pods/exec` into `claims-toolbox`, whose namespace-scoped SA alone may `patch` Deployments. Don't widen the runner's RBAC to remove that hop.

## Docs

`instructions/README.md` holds the automated-vs-manual matrix. Update it when something moves between Terraform and manual.
