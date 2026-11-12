# Incident workflows: exact names, descriptions and triggers for all four.
#
# Steps are owned in two different places:
#
# - "managed" workflows (Major Incident, Paige Virtual Responder): Terraform
#   owns the steps. Every action ID below was checked against this account's
#   GET /incident_workflows/actions, and the input formats were copied from
#   existing workflows on the account.
# - "manual" workflows (Kubernetes diagnostics / remediation): their steps go
#   through the Kubernetes Agent Connector and the self-hosted runner, which
#   the provider can't look up. You add them in the UI
#   (instructions/05-incident-workflows.md), and `ignore_changes = [step]`
#   stops a later apply from wiping them.

# Major Incident sets this priority when a person declares it.
data "pagerduty_priority" "p1" {
  count = var.enable_priority ? 1 : 0
  name  = var.priority_name
}

locals {
  responder_ref = [{ id = data.pagerduty_user.responder.id, type = "user_reference" }]
  stakeholders = concat(
    local.responder_ref,
    local.team == null ? [] : [{ id = local.team, type = "team_reference" }],
  )
  use_zoom  = var.major_incident_conference == "zoom"
  use_slack = var.slack_workspace_id != ""

  managed_workflows = var.manage_incident_workflows ? {
    paige_slack = {
      name        = "Paige Virtual Responder - Dedicated Slack Channel"
      description = "Creates a dedicated Slack channel for the incident and brings in Paige to investigate."
      trigger     = "conditional"
      services    = [for s in pagerduty_service.tech : s.id]
      steps = concat(
        local.use_slack ? [{
          name   = "Create Incident Dedicated Channel" # Add Paige's defaults reference this exact name
          action = "pagerduty.com:slack:create-a-channel:4"
          inputs = { "Workspace" = var.slack_workspace_id }
        }] : [],
        [{
          name   = "Add Paige"
          action = "pagerduty.com:pagerduty-advance:auto-triage:5"
          inputs = {
            "Post in Paige in Pagerduty"           = "true"
            "Post investigation results as a Note" = "true"
            "Post investigation results in Slack"  = local.use_slack ? "true" : "false"
          }
        }],
      )
    }
    major_incident = {
      name        = "Major Incident"
      description = "For P1 incidents with customer impact. Adds responders and stakeholders, opens a Zoom bridge and brings in Scribe to capture decisions."
      trigger     = "manual"
      services    = [for s in pagerduty_service.tech : s.id]
      steps = concat(
        var.enable_priority ? [{
          name   = "Set priority ${var.priority_name}"
          action = "pagerduty.com:incident-workflows:update-incident-priority:1"
          inputs = { "Priority" = data.pagerduty_priority.p1[0].id }
        }] : [],
        [
          {
            name   = "Add responders"
            action = "pagerduty.com:incident-workflows:add-responders:2"
            inputs = { "Responders" = jsonencode(local.responder_ref) }
          },
          {
            name   = "Add stakeholders"
            action = "pagerduty.com:incident-workflows:add-stakeholders:3"
            inputs = { "Stakeholders" = jsonencode(local.stakeholders) }
          },
        ],
        local.use_zoom ? [
          {
            name   = "Create a Zoom meeting"
            action = "pagerduty.com:zoom:create-zoom-meeting:1"
            inputs = {}
          },
          {
            name   = "Add Scribe"
            action = "pagerduty.com:pagerduty-advance:add-scribe-agent:1"
            inputs = {}
          },
        ] : [],
      )
    }
  } : {}

  manual_workflows = var.manage_incident_workflows ? {
    k8s_diagnostics = {
      name        = "Kubernetes diagnostics"
      description = "Lists the claims-reconciliation pods through the Kubernetes Agent Connector and sends them to Paige as an incident note."
      trigger     = "conditional"
      services    = [pagerduty_service.tech["claims"].id]
    }
    k8s_remediation = {
      name        = "Kubernetes Remediation"
      description = "Restarts the claims-reconciliation pod and applies a memory limit to the deployment. Run only after a person approves Paige's recommendation."
      trigger     = "manual"
      services    = [pagerduty_service.tech["claims"].id]
    }
  } : {}

  all_workflows = merge(local.managed_workflows, local.manual_workflows)
}

resource "pagerduty_incident_workflow" "managed" {
  for_each = local.managed_workflows

  name        = each.value.name
  description = each.value.description
  team        = local.team

  dynamic "step" {
    for_each = each.value.steps
    content {
      name   = step.value.name
      action = step.value.action
      dynamic "input" {
        for_each = step.value.inputs
        content {
          name  = input.key
          value = input.value
        }
      }
    }
  }
}

resource "pagerduty_incident_workflow" "manual" {
  for_each = local.manual_workflows

  name        = each.value.name
  description = each.value.description
  team        = local.team

  lifecycle {
    ignore_changes = [step]
  }
}

resource "pagerduty_incident_workflow_trigger" "this" {
  for_each = local.all_workflows

  workflow                   = try(pagerduty_incident_workflow.managed[each.key].id, pagerduty_incident_workflow.manual[each.key].id)
  type                       = each.value.trigger
  services                   = each.value.services
  subscribed_to_all_services = false
  condition                  = each.value.trigger == "conditional" ? var.conditional_trigger_expression : null
}

# The workflows used to be one resource; keep the existing ones (same IDs).
moved {
  from = pagerduty_incident_workflow.this["paige_slack"]
  to   = pagerduty_incident_workflow.managed["paige_slack"]
}

moved {
  from = pagerduty_incident_workflow.this["major_incident"]
  to   = pagerduty_incident_workflow.managed["major_incident"]
}

moved {
  from = pagerduty_incident_workflow.this["k8s_diagnostics"]
  to   = pagerduty_incident_workflow.manual["k8s_diagnostics"]
}

moved {
  from = pagerduty_incident_workflow.this["k8s_remediation"]
  to   = pagerduty_incident_workflow.manual["k8s_remediation"]
}
