# Optional. Everything here is also in the menu: just run ./trigger
TF := terraform -chdir=terraform

.DEFAULT_GOAL := help
.PHONY: help build test dry-run plan create run resolve destroy fmt validate kind-up kind-down k8s-up k8s-down k8s-reset

help: ## List targets (or skip make entirely: ./trigger)
	@grep -hE '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-11s %s\n", $$1, $$2}'

build: ## Build the app into bin/ (./trigger does this automatically)
	cd app && go build -o ../bin/trigger .

test: ## Vet the app and render every payload (catches broken edits in payloads/)
	cd app && go vet ./... && go test ./...

dry-run: ## Print every rendered event without sending
	./trigger run -dry-run -no-wait

plan: ## Setup questions, then terraform plan (read-only)
	./trigger plan

create: ## Setup questions, then terraform apply
	./trigger create

run: ## Send the scenario: change event, then alerts
	./trigger run

resolve: ## Resolve every scenario alert
	./trigger resolve

destroy: ## Resolve, then terraform destroy
	./trigger destroy

fmt: ## Format Go and Terraform
	cd app && gofmt -w .
	$(TF) fmt

validate: ## Offline checks: terraform validate + go test
	$(TF) init -backend=false -input=false >/dev/null && $(TF) validate
	$(MAKE) test

k8s-up: ## Create the kind cluster if needed, apply workloads
	./trigger cluster up

kind-up: k8s-up

k8s-reset: ## Recreate claims-reconciliation without a memory limit (between demos)
	./trigger cluster reset

kind-down: ## Delete the kind cluster
	./trigger cluster down

k8s-down: ## Remove the healthcare namespace only
	kubectl --context kind-reinvent delete namespace healthcare --ignore-not-found
