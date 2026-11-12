package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// pdObject is something Terraform creates, identified by kind and exact name.
type pdObject struct{ kind, name string }

// ownedObjects are the names Terraform gives the objects it creates
// (variables.tf defaults and workflows.tf). TestOwnedObjectsMatchTerraform
// keeps this list honest.
var ownedObjects = []pdObject{
	{"service", "claims-reconciliation"},
	{"service", "scheduling"},
	{"business service", "patient-portal"},
	{"escalation policy", "claims-reconciliation escalation"},
	{"event orchestration", "Reinvent Patient"},
	{"incident workflow", "Paige Virtual Responder - Dedicated Slack Channel"},
	{"incident workflow", "Kubernetes diagnostics"},
	{"incident workflow", "Major Incident"},
	{"incident workflow", "Kubernetes Remediation"},
}

// preflight checks the account before Terraform touches it and returns the
// extra -var flags it needs. With checkConflicts it refuses to go on when
// the account already holds objects with our names that we didn't create.
func preflight(root string, s settings, checkConflicts bool) ([]string, error) {
	vars := []string{"-var=responder_email=" + s.email}

	var pr struct {
		Priorities []json.RawMessage `json:"priorities"`
	}
	if _, err := apiGet(s.token, s.region, "/priorities", &pr); err == nil && len(pr.Priorities) == 0 {
		fmt.Println("  Note: priorities are switched off on this account, so Major Incident won't set one.")
		vars = append(vars, "-var=enable_priority=false")
	}

	if !checkConflicts {
		return vars, nil
	}
	owned := stateIDs(filepath.Join(root, "terraform", "terraform.tfstate"))
	var clashes []string
	for _, o := range ownedObjects {
		id, link, err := findByName(s, o)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: couldn't check for an existing %s %q: %v\n", o.kind, o.name, err)
			continue
		}
		if id != "" && !owned[id] {
			clashes = append(clashes, fmt.Sprintf("  - %s %q  %s", o.kind, o.name, link))
		}
	}
	if len(clashes) > 0 {
		return nil, fmt.Errorf("your PagerDuty account already has these, and this copy of Reinvent didn't create them:\n%s\n\n"+
			"Fix: delete or rename them in PagerDuty, then try again.\n"+
			"If you set Reinvent up on another computer, run \"Delete PagerDuty objects\" there first.",
			strings.Join(clashes, "\n"))
	}
	return vars, nil
}

// findByName returns the ID (and a link) of an object of this kind with exactly this name.
func findByName(s settings, o pdObject) (id, link string, err error) {
	q := url.QueryEscape(o.name)
	type named struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		HTMLURL string `json:"html_url"`
	}
	match := func(list []named) (string, string) {
		for _, x := range list {
			if strings.EqualFold(x.Name, o.name) {
				return x.ID, x.HTMLURL
			}
		}
		return "", ""
	}
	switch o.kind {
	case "service":
		var r struct{ Services []named }
		_, err = apiGet(s.token, s.region, "/services?limit=100&query="+q, &r)
		id, link = match(r.Services)
	case "escalation policy":
		var r struct {
			Items []named `json:"escalation_policies"`
		}
		_, err = apiGet(s.token, s.region, "/escalation_policies?limit=100&query="+q, &r)
		id, link = match(r.Items)
	case "incident workflow":
		var r struct {
			Items []named `json:"incident_workflows"`
		}
		_, err = apiGet(s.token, s.region, "/incident_workflows?limit=100&query="+q, &r)
		id, link = match(r.Items)
	case "business service":
		for off := 0; ; off += 100 {
			var r struct {
				Items []named `json:"business_services"`
				More  bool    `json:"more"`
			}
			if _, err = apiGet(s.token, s.region, fmt.Sprintf("/business_services?limit=100&offset=%d", off), &r); err != nil {
				break
			}
			if id, link = match(r.Items); id != "" || !r.More {
				break
			}
		}
	case "event orchestration":
		for off := 0; ; off += 100 {
			var r struct {
				Items []named `json:"orchestrations"`
				More  bool    `json:"more"`
			}
			if _, err = apiGet(s.token, s.region, fmt.Sprintf("/event_orchestrations?limit=100&offset=%d", off), &r); err != nil {
				break
			}
			if id, link = match(r.Items); id != "" || !r.More {
				break
			}
		}
	}
	return id, link, err
}

// stateIDs returns every resource ID in the local Terraform state.
func stateIDs(path string) map[string]bool {
	ids := map[string]bool{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ids
	}
	var st struct {
		Resources []struct {
			Instances []struct {
				Attributes struct {
					ID string `json:"id"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(raw, &st) == nil {
		for _, r := range st.Resources {
			for _, i := range r.Instances {
				ids[i.Attributes.ID] = true
			}
		}
	}
	return ids
}

// planAndApply runs Terraform quietly, shows a plain-English summary of what
// will change, asks y/N (unless auto), then applies exactly that plan.
// previewOnly stops after the summary (menu option 2).
func planAndApply(dir string, vars []string, destroy, auto, previewOnly bool) error {
	fmt.Println("Talking to PagerDuty…")
	if out, err := terraformQuiet(dir, "init", "-input=false"); err != nil {
		return tfFailed("getting Terraform ready", out, err)
	}
	planFile := filepath.Join(dir, "reinvent.tfplan")
	defer os.Remove(planFile)
	args := append([]string{"plan", "-input=false", "-out=" + planFile}, vars...)
	if destroy {
		args = append(args, "-destroy")
	}
	if out, err := terraformQuiet(dir, args...); err != nil {
		return tfFailed("checking what to change", out, err)
	}
	showOut, err := terraformQuiet(dir, "show", "-json", planFile)
	if err != nil {
		return tfFailed("reading the plan", showOut, err)
	}
	creates, updates, deletes := summarisePlan([]byte(showOut))
	if len(creates)+len(updates)+len(deletes) == 0 {
		fmt.Println("\nEverything in PagerDuty is already up to date. Nothing to do.")
		return nil
	}
	fmt.Println()
	printGroup := func(title, colour string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Printf("%s%s (%d):%s\n", colour, title, len(items), cReset)
		for _, it := range items {
			fmt.Println("   ", it)
		}
	}
	printGroup("Will create", cGreen, creates)
	printGroup("Will update", cAccent, updates)
	printGroup("Will DELETE", cRed, deletes)
	fmt.Println()
	if previewOnly {
		fmt.Println("This was a preview. Nothing has been changed.")
		return nil
	}
	if !auto {
		if !isTerminal() {
			return errors.New("not a terminal: pass -auto-approve to apply without asking")
		}
		ok, err := askYesNo("Go ahead?", len(deletes) == 0)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Cancelled. Nothing was changed.")
			return nil
		}
	}
	fmt.Println("Applying… (this takes under a minute)")
	if out, err := terraformQuiet(dir, "apply", "-input=false", planFile); err != nil {
		return tfFailed("making the changes", out, err)
	}
	return nil
}

var friendlyKinds = map[string]string{
	"pagerduty_service":                     "service",
	"pagerduty_business_service":            "business service",
	"pagerduty_service_dependency":          "service dependency",
	"pagerduty_service_integration":         "integration",
	"pagerduty_escalation_policy":           "escalation policy",
	"pagerduty_alert_grouping_setting":      "alert grouping",
	"pagerduty_event_orchestration":         "event orchestration",
	"pagerduty_event_orchestration_router":  "orchestration routing",
	"pagerduty_event_orchestration_global":  "orchestration rules (runbook link)",
	"pagerduty_event_orchestration_service": "service orchestration rules",
	"pagerduty_incident_workflow":           "incident workflow",
	"pagerduty_incident_workflow_trigger":   "workflow trigger",
}

func summarisePlan(showJSON []byte) (creates, updates, deletes []string) {
	var p struct {
		ResourceChanges []struct {
			Type   string `json:"type"`
			Key    any    `json:"index"`
			Change struct {
				Actions []string       `json:"actions"`
				Before  map[string]any `json:"before"`
				After   map[string]any `json:"after"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if json.Unmarshal(showJSON, &p) != nil {
		return
	}
	for _, rc := range p.ResourceChanges {
		kind := friendlyKinds[rc.Type]
		if kind == "" {
			kind = strings.TrimPrefix(rc.Type, "pagerduty_")
		}
		label := kind
		for _, m := range []map[string]any{rc.Change.After, rc.Change.Before} {
			if n, ok := m["name"].(string); ok && n != "" {
				label = fmt.Sprintf("%s %q", kind, n)
				break
			}
		}
		if label == kind && rc.Key != nil {
			label = fmt.Sprintf("%s (%v)", kind, rc.Key)
		}
		a := strings.Join(rc.Change.Actions, ",")
		switch {
		case a == "create":
			creates = append(creates, label)
		case a == "update":
			updates = append(updates, label)
		case a == "delete":
			deletes = append(deletes, label)
		case strings.Contains(a, "delete") && strings.Contains(a, "create"):
			deletes = append(deletes, label+" (then re-created)")
		}
	}
	sort.Strings(creates)
	sort.Strings(updates)
	sort.Strings(deletes)
	return
}

// terraformQuiet runs terraform and returns its combined output instead of
// streaming it, so users see a summary rather than pages of Terraform.
func terraformQuiet(dir string, args ...string) (string, error) {
	tf, err := tool("terraform")
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	// init, plan, show and apply all accept -no-color after the subcommand.
	cmd := exec.Command(tf, append([]string{"-chdir=" + dir, args[0], "-no-color"}, args[1:]...)...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err = cmd.Run()
	return buf.String(), err
}

// tfFailed turns a Terraform failure into a plain message with a fix.
func tfFailed(what, out string, err error) error {
	low := strings.ToLower(out)
	hint := "The details are below. If you're stuck, send them to whoever gave you Reinvent."
	switch {
	case strings.Contains(low, "401") || strings.Contains(low, "unauthorized"):
		hint = "PagerDuty rejected the API key. Choose \"PagerDuty settings\" (1) and enter a new key, and check the region (US/EU)."
	case strings.Contains(low, "403") || strings.Contains(low, "forbidden") || strings.Contains(low, "access denied"):
		hint = "The API key isn't allowed to do this. Use an account-level REST API key with read/write access (Integrations → API Access Keys), then choose \"PagerDuty settings\" (1)."
	case strings.Contains(low, "already") && (strings.Contains(low, "taken") || strings.Contains(low, "exists")):
		hint = "Something with the same name already exists in PagerDuty. Delete or rename it there, then try again."
	case strings.Contains(low, "no such host") || strings.Contains(low, "timeout") || strings.Contains(low, "connection refused"):
		hint = "Couldn't reach PagerDuty or the Terraform registry. Check your internet connection or VPN."
	case strings.Contains(low, "state lock") || strings.Contains(low, "lock"):
		hint = "Another Reinvent window is changing PagerDuty right now. Close it, wait a minute, then try again."
	}
	tail := strings.TrimSpace(out)
	if lines := strings.Split(tail, "\n"); len(lines) > 25 {
		tail = strings.Join(lines[len(lines)-25:], "\n")
	}
	return fmt.Errorf("failed while %s.\n\n  %s\n\n%s%s%s\n(%v)", what, hint, cDim, tail, cReset, err)
}
