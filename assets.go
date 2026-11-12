// Package reinvent embeds everything the distributed app needs at run time, so
// the single downloaded binary works without a checkout of this repo.
package reinvent

import "embed"

// Files are extracted into the app's workspace on start (see app/workspace.go).
//
//go:embed payloads k8s runbook terraform/*.tf terraform/.terraform.lock.hcl terraform/terraform.tfvars.example
var Files embed.FS
