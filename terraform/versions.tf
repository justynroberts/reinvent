terraform {
  required_version = ">= 1.5.0"

  required_providers {
    pagerduty = {
      source  = "PagerDuty/pagerduty"
      version = "~> 3.26"
    }
  }
}

# Credentials come from the environment, never from this repo:
#   PAGERDUTY_TOKEN           - REST API key (account-level, read/write)
#   PAGERDUTY_SERVICE_REGION  - "eu" for EU accounts, unset for US
provider "pagerduty" {}
