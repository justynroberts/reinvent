package main

import (
	"fmt"
	"net/http"
	"time"
)

// cmdDoctor checks everything a demo needs and fixes what it can (it
// downloads Terraform if missing). Safe to run any time: it changes nothing
// in PagerDuty.
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
	check("PagerDuty settings (region, API key, email)", err, "In the menu: SETUP → 1 PagerDuty settings.")
	_ = s

	_, err = tool("terraform")
	check("Terraform (downloaded automatically)", err, "")

	fmt.Println()
	if ok {
		fmt.Println("Everything's ready.")
		return nil
	}
	return fmt.Errorf("fix the items marked ✗ above")
}
