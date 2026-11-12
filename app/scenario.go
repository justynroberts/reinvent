package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Scenario is payloads/scenario.json: shared template vars plus ordered steps.
type Scenario struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Vars        map[string]string `json:"vars"`
	Steps       []Step            `json:"steps"`

	dir string // payload paths are relative to the scenario file
}

type Step struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // "alert" or "change"
	Payload   string `json:"payload"`    // path relative to the scenario file
	WaitAfter string `json:"wait_after"` // Go duration, e.g. "15s"
}

type runOptions struct {
	noWait bool
	only   string
}

func loadScenario(path string) (*Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sc Scenario
	if err := json.Unmarshal(raw, &sc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sc.dir = filepath.Dir(path)
	for i, st := range sc.Steps {
		if st.Type != "alert" && st.Type != "change" {
			return nil, fmt.Errorf("step %d (%s): type must be alert or change, got %q", i+1, st.Name, st.Type)
		}
		if st.WaitAfter != "" {
			if _, err := time.ParseDuration(st.WaitAfter); err != nil {
				return nil, fmt.Errorf("step %d (%s): wait_after: %w", i+1, st.Name, err)
			}
		}
	}
	return &sc, nil
}

// render executes a payload template and parses the result as a JSON object.
func (sc *Scenario) render(st Step) (map[string]any, error) {
	path := filepath.Join(sc.dir, st.Payload)
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	funcs := template.FuncMap{
		"now": func() string { return now.Format(time.RFC3339) },
		"ago": func(d string) (string, error) {
			dur, err := time.ParseDuration(d)
			if err != nil {
				return "", err
			}
			return now.Add(-dur).Format(time.RFC3339), nil
		},
	}
	tmpl, err := template.New(st.Payload).Funcs(funcs).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", st.Payload, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Vars map[string]string }{sc.Vars}); err != nil {
		return nil, fmt.Errorf("%s: %w", st.Payload, err)
	}
	var body map[string]any
	if err := json.Unmarshal(buf.Bytes(), &body); err != nil {
		return nil, fmt.Errorf("%s: rendered payload is not valid JSON (a var containing a quote?): %w", st.Payload, err)
	}
	return body, nil
}

func (sc *Scenario) Run(s *sender, opt runOptions) error {
	fmt.Printf("Scenario: %s\n", sc.Name)
	sent := 0
	for i, st := range sc.Steps {
		if opt.only != "" && !strings.Contains(strings.ToLower(st.Name), strings.ToLower(opt.only)) {
			continue
		}
		body, err := sc.render(st)
		if err != nil {
			return err
		}
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(sc.Steps), st.Name, st.Type)
		if err := s.send(st.Type, body); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, st.Name, err)
		}
		sent++
		if wait, _ := time.ParseDuration(st.WaitAfter); wait > 0 && !opt.noWait && !s.dryRun && i < len(sc.Steps)-1 {
			fmt.Printf("    waiting %s\n", wait)
			time.Sleep(wait)
		}
	}
	if sent == 0 {
		return fmt.Errorf("no step name contains %q", opt.only)
	}
	return nil
}

// Resolve sends a resolve for the dedup_key of every alert step. It sends the
// whole rendered payload, not just the key, so the global orchestration's
// dynamic routing (custom_details.service) delivers it to the same service.
func (sc *Scenario) Resolve(s *sender) error {
	for _, st := range sc.Steps {
		if st.Type != "alert" {
			continue
		}
		body, err := sc.render(st)
		if err != nil {
			return err
		}
		key, _ := body["dedup_key"].(string)
		if key == "" {
			fmt.Printf("skip %s: no dedup_key in payload\n", st.Name)
			continue
		}
		fmt.Printf("resolve %s (service %s)\n", key, routedTo(body))
		body["event_action"] = "resolve"
		if err := s.send("alert", body); err != nil {
			return err
		}
	}
	return nil
}
