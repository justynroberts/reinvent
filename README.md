# reinvent: claims-reconciliation incident demo

A PagerDuty demo. A library upgrade makes the `claims-reconciliation` pod leak memory. The pod starves its Kubernetes node, which takes `patient-portal` and `scheduling` down and backs up claims with compliance exposure. Paige finds the change, a human approves, and workflows fix it in-cluster (restart the pod and apply a memory limit).

## Install (Mac)

Open **Terminal** (Cmd+Space, type Terminal), paste this line and press Return:

```
curl -fsSL https://raw.githubusercontent.com/justynroberts/reinvent/main/install.sh | sh
```

Reinvent opens in a Terminal window. Next time, open it from Spotlight (Cmd+Space, type **Reinvent**).

**What you need:**
- A **PagerDuty API key** (PagerDuty → Integrations → API Access Keys). Reinvent asks for it, plus your region and login email, and checks each one.
- **Docker Desktop** ([free download](https://www.docker.com/products/docker-desktop/)), only for the Kubernetes cluster steps.

That's all. Reinvent downloads Terraform, kind and kubectl itself the first time it needs them. Choose **Check this Mac is ready** in the menu to confirm everything is in place. Your settings and state live in `~/Library/Application Support/Reinvent`.

<details><summary>Prefer to download the file instead?</summary>

Download `Reinvent-macos.zip` from [the latest release](https://github.com/justynroberts/reinvent/releases/latest), unzip it, and move `Reinvent.app` to Applications. The app isn't notarised by Apple, so the first time you open it macOS blocks it. Go to System Settings → Privacy & Security, scroll down, and click **Open Anyway**. The Terminal install above avoids this.
</details>

## Layout

| Dir | What |
|---|---|
| [`story/`](story/) | The narrative and demo beats (`README.md`), plus the original brief (`brief.md`) |
| [`instructions/`](instructions/) | Platform setup, in order, with the **automated vs manual matrix** |
| [`terraform/`](terraform/) | Services, business services, dependencies, EP, alert grouping, Event Orchestration, workflow shells |
| [`payloads/`](payloads/) | `scenario.json` (order, timing, shared vars) and the event templates. Edit these, not code. |
| [`trigger`](trigger) | Developers: run from a checkout. `./trigger` opens the same menu, using the repo's files |
| [`app/`](app/) | Go source (rebuilt automatically by `./trigger` when it changes) |
| [`scripts/release.sh`](scripts/release.sh) | Builds, verifies and (with `--publish`) releases `Reinvent.app` |
| [`runbook/`](runbook/) | The claims-reconciliation runbook to upload for Paige |
| [`k8s/`](k8s/) | kind cluster, leaky demo workloads, remediation toolbox + RBAC |

## Quick start

You need Go, Terraform, and (for the cluster) Docker, kind and kubectl. `./trigger` tells you the install command for anything missing.

```bash
./trigger
```

That's it. A full-screen menu opens with two pages: **SETUP** (do once) and **DEMO** (use on stage). Switch with ← →. Once setup is done it opens straight on DEMO. ★ marks the next step. It covers everything:
settings (region US/EU, API key, your email, each checked against PagerDuty and saved to `.env`),
creating the PagerDuty objects, the local cluster, running, resolving, resetting between demos, and teardown.
Anything that sends alerts or deletes something asks first.

Between `create` and the first run, do the manual steps in [instructions/](instructions/).

For scripts: `./trigger help` lists the subcommands (`setup`, `plan`, `create`, `run`, `resolve`, `destroy`, `cluster up|reset|down`).

## Editing the scenario

- **Wording, severities, custom details:** edit `payloads/alerts/*.json` or `payloads/change-event.json`. They are Go templates with `{{.Vars.x}}`, `{{now}}` and `{{ago "47m"}}`.
- **Order, timing, adding or removing an event:** edit `steps` in `payloads/scenario.json`.
- **Cluster, node, namespace, versions, library:** edit `vars` in `payloads/scenario.json`.
- **Then run `make test`** (or `./trigger run -dry-run`). It renders every step and checks that each one is a valid event.

The routing key is injected at send time from `terraform output routing_key` (or `PD_ROUTING_KEY`). It never goes in a payload file.

## Can and can't be automated

The short version, with the full matrix in [instructions/README.md](instructions/README.md):

- 🟢 **Automated:** services, business services, dependencies, routing key, change events, payloads, alert grouping, EO rule, EP (human), workflow names/descriptions/triggers, k8s workloads.
- 🔴 **Manual:** Paige on the EP, workflow steps, runner registration, Agent Connector, Slack/Paige scopes, Zoom/Meet, runbook upload, internal status page, console view, EA flags.
