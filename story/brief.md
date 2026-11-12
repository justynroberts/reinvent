# Original brief (source, unedited)

Kept as received. `story/README.md` is the version written for presenting.

---

## The Incident

A hung process in the claims-reconciliation pod is leaking memory and has taken down three services on the same Kubernetes node. Patient portal and scheduling are down, claims submissions are backing up, and there's compliance exposure. Root cause: a memory leak since a library upgrade. Fix: Restart the pod and apply a memory limit.

- Core Demo Flow can be configured in individual sandboxes
- Subdomain is on a PD Reliability Platform plan
- Familiarize yourself with the left vs. top nav
- Console view saved with the Paige column + Alerts field
- EA flags to be enabled: Paige on Escalation Policies, PD Automation Runner

## Setup Checklist

Set up 3 services

- Technical service: claims-reconciliation
- Business services: patient-portal + scheduling

### Payload content

- Integration to send event with tool of choice (eg Postman, cURL, etc) and event payload that matches incident
- Must mention at least Kubernetes for EO rule and have enough context for Paige to determine root cause

### Integrations and Connectors

- Slack integration connected, PagerDuty AI connected to Slack, and the integration reauthorized to grant Paige's scopes
- Zoom / Google Meet integration connected
- PD Automation Runner deployed as a pod in the demo cluster, RBAC applied, and showing Healthy under Incident Workflows → Automation Connectors → Self-hosted Runners.
  - If you do not have a Kubernetes cluster in your sandbox, you will need to run Kind locally on your laptop and deploy the runner there. Page: UA Runner Kind Cluster Management Script
  - PD Automation Runner KB
- Agent Connector for Kubernetes with all tools enabled
- Change Events on Service

### Configurations

- Escalation Policy: Paige at level 1 (Trigger SRE Agent toggle on) and you as the human responder
- Intelligent Alert Grouping enabled
- Event Orchestration Rule
  - Example: https://pdt-orbitpay.pagerduty.com/event-orchestration/service/PT03CRX
- Configure these Incident Workflows (build from templates), published with these exact names:
  - **Paige Virtual Responder - Dedicated Slack Channel**
    - Trigger: Conditional on incident creation.
    - Description: Creates a dedicated Slack channel for the incident and brings in Paige to investigate.
    - Actions to add: Under Surface, select Slack > When an incident is triggered on your demo service, Paige automatically creates a dedicated Slack channel and posts its findings and recommended next steps.
  - **Kubernetes diagnostics**
    - Trigger: Conditional on incident creation.
    - Actions to add, in order:
      - List Objects, renamed List claims-reconciliation pods
      - Add Note to an Incident, renamed Send Pods to Paige
    - Example: https://pdt-orbitpay.pagerduty.com/incident-workflows/workflows/PU7B4A9/edit
  - **Major Incident**
    - Trigger: Manual (runs only when Paige recommends it and a person approves)
    - Description: For P1 incidents with customer impact. Adds responders and stakeholders, opens a Zoom bridge and brings in Scribe to capture decisions.
    - Actions to add, in order: Add Responders > Add Stakeholders > Create a Zoom Meeting > Add Scribe Agent
  - **Kubernetes Remediation**
    - Trigger: Manual
    - Actions to add
      - Pod Created through Deployment: Delete Pod, Delay, Pod Command to update memory limit
      - Pod Created Adhoc: Delete Pod, Delay, Create Pod to mimic K8s spinning up new pod from deployment
    - Example: https://pdt-orbitpay.pagerduty.com/incident-workflows/workflows/PNAX9NY/edit
- Runbook uploaded for the claims-reconciliation service, with the fix being restart the pod + apply a memory limit to the claims-reconciliation deployment
- Internal Status Page: Configured to show those two business services

## Enablement Sessions to Check

- Paige and Automation
- MCP
- SRE Workbench (PD Labs)
