package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPayloadsRender guards the externalised payloads: after editing any file in
// payloads/, `go test ./...` proves every step still renders to a valid event.
func TestPayloadsRender(t *testing.T) {
	sc, err := loadScenario(filepath.Join("..", "payloads", "scenario.json"))
	if err != nil {
		t.Fatal(err)
	}
	severities := map[string]bool{"critical": true, "error": true, "warning": true, "info": true}
	changes := 0
	services := map[string]bool{}
	for _, v := range []string{"claims_service", "scheduling_service"} {
		if sc.Vars[v] == "" {
			t.Fatalf("scenario vars.%s is required (must match the Terraform service name)", v)
		}
		services[sc.Vars[v]] = true
	}
	for _, st := range sc.Steps {
		body, err := sc.render(st)
		if err != nil {
			t.Fatalf("%s: %v", st.Name, err)
		}
		p, ok := body["payload"].(map[string]any)
		if !ok {
			t.Fatalf("%s: missing payload object", st.Name)
		}
		if s, _ := p["summary"].(string); s == "" || len(s) > 1024 {
			t.Errorf("%s: summary must be 1-1024 chars", st.Name)
		}
		if src, _ := p["source"].(string); src == "" {
			t.Errorf("%s: payload.source is required", st.Name)
		}
		switch st.Type {
		case "change":
			changes++
		case "alert":
			if a, _ := body["event_action"].(string); a != "trigger" {
				t.Errorf("%s: event_action must be trigger", st.Name)
			}
			if k, _ := body["dedup_key"].(string); k == "" {
				t.Errorf("%s: dedup_key required so `trigger resolve` works", st.Name)
			}
			if svc := routedTo(body); !services[svc] {
				t.Errorf("%s: custom_details.service %q must be one of the technical services %v (dynamic routing looks it up by name)", st.Name, svc, services)
			}
			if sev, _ := p["severity"].(string); !severities[sev] {
				t.Errorf("%s: severity %q invalid", st.Name, sev)
			}
		}
	}
	if changes == 0 {
		t.Error("scenario has no change event; Paige needs it to find the library upgrade")
	}
}

// TestServiceNamesMatchTerraform: dynamic routing looks services up by exact
// name, so the payload vars must equal the Terraform defaults.
func TestServiceNamesMatchTerraform(t *testing.T) {
	sc, err := loadScenario(filepath.Join("..", "payloads", "scenario.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "terraform", "variables.tf"))
	if err != nil {
		t.Fatal(err)
	}
	for tfVar, scVar := range map[string]string{
		"technical_service_name":  "claims_service",
		"scheduling_service_name": "scheduling_service",
		"business_service_name":   "business_service",
	} {
		re := regexp.MustCompile(`(?s)variable "` + tfVar + `" \{.*?default\s*=\s*"([^"]*)"`)
		m := re.FindSubmatch(raw)
		if m == nil {
			t.Errorf("terraform/variables.tf: no default for %s", tfVar)
			continue
		}
		if got := sc.Vars[scVar]; got != string(m[1]) {
			t.Errorf("payloads vars.%s = %q but terraform %s = %q", scVar, got, tfVar, m[1])
		}
	}
}

// TestOwnedObjectsMatchTerraform: the preflight name-collision check must
// look for exactly the names Terraform creates.
func TestOwnedObjectsMatchTerraform(t *testing.T) {
	var all []byte
	for _, f := range []string{"main.tf", "variables.tf", "workflows.tf", "orchestration.tf"} {
		b, err := os.ReadFile(filepath.Join("..", "terraform", f))
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, b...)
	}
	tf := strings.ReplaceAll(string(all), "${var.technical_service_name}", "claims-reconciliation")
	for _, o := range ownedObjects {
		if !strings.Contains(tf, `"`+o.name+`"`) && !strings.Contains(tf, `"`+o.name) {
			t.Errorf("ownedObjects has %s %q, which isn't a name in terraform/*.tf", o.kind, o.name)
		}
	}
}
