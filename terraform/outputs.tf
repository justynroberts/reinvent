output "routing_key" {
  description = "Global Event Orchestration routing key. Every alert goes here and is routed on custom_details.service. Read by the trigger app."
  value       = pagerduty_event_orchestration.healthcare.integration[0].parameters[0].routing_key
  sensitive   = true
}

output "change_routing_keys" {
  description = "Change-event integration key per technical service name (/v2/change/enqueue). The app picks one by the change event's custom_details.service."
  value       = { for k, i in pagerduty_service_integration.changes : pagerduty_service.tech[k].name => i.integration_key }
  sensitive   = true
}

output "services" {
  description = "Technical services. Their names are the values custom_details.service must carry."
  value       = { for k, s in pagerduty_service.tech : k => { id = s.id, name = s.name, html_url = s.html_url } }
}

output "business_service" {
  value = {
    id       = pagerduty_business_service.patient_portal.id
    name     = pagerduty_business_service.patient_portal.name
    html_url = pagerduty_business_service.patient_portal.html_url
  }
}

output "event_orchestration_id" {
  value = pagerduty_event_orchestration.healthcare.id
}

output "escalation_policy_id" {
  value = pagerduty_escalation_policy.claims.id
}

output "incident_workflows" {
  description = "Workflow IDs. Major Incident and the Paige workflow have their steps from Terraform; add the Kubernetes ones in the UI."
  value       = { for k, w in merge(pagerduty_incident_workflow.managed, pagerduty_incident_workflow.manual) : w.name => w.id }
}
