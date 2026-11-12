variable "responder_email" {
  description = "Email of the existing PagerDuty user who is the human responder on the escalation policy (you)."
  type        = string
}

variable "team_id" {
  description = "Optional team ID to own the services, business services and workflows. Empty = no team."
  type        = string
  default     = ""
}

# Names are fixed by the story and the demo script. Change them only if your
# sandbox already holds objects with these names.
variable "technical_service_name" {
  description = "The culprit. Must equal custom_details.service in payloads routed to it."
  type        = string
  default     = "claims-reconciliation"
}

variable "scheduling_service_name" {
  description = "Second technical service, on the same node. Must equal custom_details.service in payloads routed to it."
  type        = string
  default     = "scheduling"
}

variable "business_service_name" {
  description = "Customer-facing business service that depends on both technical services."
  type        = string
  default     = "patient-portal"
}

# --- Alert grouping ---------------------------------------------------------

variable "alert_grouping_type" {
  description = <<-EOT
    "intelligent" (the story), "content_based" (deterministic: groups on alert source, i.e. the
    Kubernetes node) or "time". Intelligent grouping learns from history, so a brand-new service
    may not group the first run. Switch to content_based for a guaranteed single incident.
  EOT
  type        = string
  default     = "intelligent"

  validation {
    condition     = contains(["intelligent", "content_based", "time"], var.alert_grouping_type)
    error_message = "alert_grouping_type must be intelligent, content_based or time."
  }
}

variable "iag_fields" {
  description = "Fields intelligent grouping compares across both services. Every alert sets source to the Kubernetes node, which is the signal they share."
  type        = list(string)
  default     = ["source"]
}

variable "alert_grouping_window_seconds" {
  description = "Grouping window. Intelligent: 300-3600. Time: timeout in seconds."
  type        = number
  default     = 900
}

# --- Event Orchestration ----------------------------------------------------

variable "enable_priority" {
  description = "Major Incident sets the incident to priority_name. Requires priorities enabled on the account. Incidents are never given a priority automatically."
  type        = bool
  default     = true
}

variable "priority_name" {
  type    = string
  default = "P1"
}

# --- Incident workflows -----------------------------------------------------

variable "manage_incident_workflows" {
  description = <<-EOT
    Create the four workflow shells (exact names, descriptions, triggers scoped to the service).
    Steps are added in the UI and ignored by Terraform. Set false if you prefer to build the
    workflows from templates entirely by hand; the names must not already exist.
  EOT
  type        = bool
  default     = true
}

variable "conditional_trigger_expression" {
  description = "PCL condition for the two conditional (on-create) workflows."
  type        = string
  default     = "incident.urgency matches 'high'"
}

variable "major_incident_conference" {
  description = "\"zoom\": Major Incident opens a Zoom meeting and adds Scribe (needs the Zoom integration connected in PagerDuty). \"none\": skip both."
  type        = string
  default     = "zoom"

  validation {
    condition     = contains(["zoom", "none"], var.major_incident_conference)
    error_message = "major_incident_conference must be zoom or none."
  }
}

variable "slack_workspace_id" {
  description = "Slack workspace ID (starts with T) connected to PagerDuty. When set, the Paige workflow creates a dedicated channel and Paige posts there; when empty, Paige posts in PagerDuty and as an incident note."
  type        = string
  default     = ""
}
