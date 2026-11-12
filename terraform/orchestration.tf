# Event Orchestration, two layers.
#
# 1. Global orchestration: every alert is sent to its routing key. Global
#    rules add a runbook link, then the router picks the service.
#    Dynamic routing reads custom_details.service and routes to the technical
#    service with that exact name. Anything without a matching name falls to
#    the catch-all, which sends it to claims-reconciliation so nothing is lost.
#
# 2. Service orchestration, on each technical service:
#    Rule 1: drop info-level probe noise (shows EO suppressing what nobody needs).
#    Rule 2: anything that mentions Kubernetes is a node-level workload problem
#            on the node patient-portal depends on. Raise to critical and
#            annotate it so Paige and responders know where to look first.
#            No priority here: a person sets P1 by running Major Incident.

resource "pagerduty_event_orchestration" "healthcare" {
  name        = "Reinvent Patient"
  description = "Kubernetes alerts for the patient-portal estate. Dynamic routing on custom_details.service to claims-reconciliation or scheduling."
  team        = local.team
}

# Global rules run before routing. This one adds a link to the runbook for the
# service the alert is about: custom_details.runbook.github =
# <runbook_base_url>/<custom_details.service>.md (runbook/*.md in this repo).
resource "pagerduty_event_orchestration_global" "healthcare" {
  event_orchestration = pagerduty_event_orchestration.healthcare.id

  set {
    id = "start"

    rule {
      label = "Add the service's runbook link (custom_details.runbook.github)"
      condition {
        expression = "event.custom_details.service exists"
      }
      actions {
        variable {
          name  = "service"
          path  = "event.custom_details.service"
          type  = "regex"
          value = "(.*)"
        }
        extraction {
          target   = "event.custom_details.runbook.github"
          template = "${var.runbook_base_url}/{{variables.service}}.md"
        }
      }
    }
  }

  catch_all {
    actions {}
  }
}

resource "pagerduty_event_orchestration_router" "healthcare" {
  event_orchestration = pagerduty_event_orchestration.healthcare.id

  set {
    id = "start"

    # Dynamic routing must be the first rule and takes no condition.
    rule {
      label = "Route by custom_details.service (exact service name)"
      actions {
        dynamic_route_to {
          lookup_by = "service_name"
          source    = "event.custom_details.service"
          regex     = "(.*)"
        }
      }
    }
  }

  catch_all {
    actions {
      route_to = pagerduty_service.tech["claims"].id
    }
  }
}

resource "pagerduty_event_orchestration_service" "tech" {
  for_each = pagerduty_service.tech

  service                                = each.value.id
  enable_event_orchestration_for_service = true

  set {
    id = "start"

    rule {
      label = "Suppress info-level Kubernetes probe noise"
      condition {
        expression = "event.severity matches 'info'"
      }
      actions {
        suppress = true
      }
    }

    rule {
      label = "Kubernetes workload behind patient-portal → critical + annotation"
      condition {
        expression = "event.summary matches regex '(?i)kubernetes' or event.custom_details.platform matches 'kubernetes'"
      }
      actions {
        severity = "critical"
        annotate = join(" ", [
          "[Event Orchestration] Kubernetes workload alert on the node shared by claims-reconciliation and scheduling; patient-portal (business service) depends on both.",
          "Check recent change events on claims-reconciliation first (library upgrade in v2.14.0).",
          "Runbook: restart the pod and apply a memory limit to the claims-reconciliation deployment.",
        ])

        extraction {
          target   = "event.custom_details.orchestration_rule"
          template = "kubernetes-node-workload"
        }
      }
    }
  }

  catch_all {
    actions {}
  }
}
