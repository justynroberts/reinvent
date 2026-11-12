package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// cmdDoctor checks everything a demo needs and fixes what it can (it
// downloads missing tools). Safe to run any time: it changes nothing in
// PagerDuty or the cluster.
func cmdDoctor() error {
	var c common
	if err := c.resolveRoot(); err != nil {
		return err
	}
	ok := true
	check := func(label string, err error, fix string) {
		if err == nil {
			fmt.Printf("  %s✓%s %s\n", cGreen, cReset, label)
			return
		}
		ok = false
		fmt.Printf("  %s✗%s %s: %v\n", cRed, cReset, label, err)
		if fix != "" {
			fmt.Printf("      %s%s%s\n", cDim, fix, cReset)
		}
	}

	fmt.Printf("Reinvent %s · files in %s\n\n", version, c.root)
	_, err := (&http.Client{Timeout: 10 * time.Second}).Head("https://api.pagerduty.com")
	check("Internet access to PagerDuty", err, "Check your connection, VPN or proxy.")

	s, err := ensureSetup(c.root)
	check("PagerDuty settings (region, API key, email)", err, "Run menu option 1.")
	_ = s

	for _, t := range []string{"terraform", "kind", "kubectl"} {
		_, err := tool(t)
		check(t+" (downloaded automatically)", err, "")
	}

	docker, err := tool("docker")
	check("Docker Desktop installed (cluster steps only)", err, "")
	if err == nil {
		err = exec.Command(docker, "info").Run()
		check("Docker Desktop running", err, "Open Docker Desktop and wait until it says it's running.")
	}

	fmt.Println()
	if ok {
		fmt.Println("Everything's ready.")
		return nil
	}
	return fmt.Errorf("fix the items marked ✗ above")
}
