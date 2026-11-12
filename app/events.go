package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// sender posts events to the PagerDuty Events API v2, or prints them in dry-run mode.
type sender struct {
	alertKey   string            // global Event Orchestration: routes on custom_details.service
	changeKeys map[string]string // service name → change integration key ("*" = any)
	dryRun     bool
	base       string
	client     *http.Client
}

func newSender(alertKey string, changeKeys map[string]string, dryRun bool) *sender {
	base := "https://events.pagerduty.com"
	if strings.EqualFold(os.Getenv("PAGERDUTY_SERVICE_REGION"), "eu") {
		base = "https://events.eu.pagerduty.com"
	}
	return &sender{alertKey: alertKey, changeKeys: changeKeys, dryRun: dryRun, base: base, client: &http.Client{Timeout: 15 * time.Second}}
}

// send injects the routing key and posts body to the alert or change endpoint.
func (s *sender) send(kind string, body map[string]any) error {
	body["routing_key"] = s.alertKey
	path := "/v2/enqueue"
	if kind == "change" {
		svc := routedTo(body)
		k, ok := s.changeKeys[svc]
		if !ok {
			k, ok = s.changeKeys["*"]
		}
		if !ok {
			return fmt.Errorf("no change-event key for service %q. In the menu, go to SETUP → 4 \"Create / update PagerDuty objects\", then try again", svc)
		}
		body["routing_key"] = k
		path = "/v2/change/enqueue"
	}

	if s.dryRun {
		shown := map[string]any{}
		for k, v := range body {
			shown[k] = v
		}
		shown["routing_key"] = "<redacted>"
		var out bytes.Buffer
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("    ", "  ")
		_ = enc.Encode(shown)
		fmt.Printf("    POST %s%s\n    %s", s.base, path, out.String())
		if kind == "change" {
			fmt.Printf("    → sent directly to the %q service's change integration\n", routedTo(body))
		} else if svc := routedTo(body); svc != "" {
			fmt.Printf("    → Reinvent Patient orchestration routes this to service %q\n", svc)
		}
		return nil
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := s.client.Post(s.base+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("%s returned %s: %s", path, resp.Status, strings.TrimSpace(string(respBody)))
	}
	var ack struct {
		Message  string `json:"message"`
		DedupKey string `json:"dedup_key"`
	}
	_ = json.Unmarshal(respBody, &ack)
	if ack.DedupKey != "" {
		fmt.Printf("    accepted (dedup_key %s)\n", ack.DedupKey)
	} else {
		fmt.Printf("    accepted\n")
	}
	return nil
}

// routedTo returns custom_details.service, which the global orchestration's
// dynamic routing uses to pick the technical service.
func routedTo(body map[string]any) string {
	p, _ := body["payload"].(map[string]any)
	cd, _ := p["custom_details"].(map[string]any)
	svc, _ := cd["service"].(string)
	return svc
}
