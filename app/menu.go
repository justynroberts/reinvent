package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type menuItem struct {
	key, label, help string
	confirm          string // non-empty: ask before running
	confirmYes       bool   // Enter means yes for this confirm
	run              func() error
}

// The menu has two pages. DEMO is what you use on stage; SETUP is everything
// you do once. The menu opens on DEMO when setup is complete.
const (
	pageSetup = iota
	pageDemo
)

var pageNames = []string{"SETUP", "DEMO"}

// ANSI styling; all empty when NO_COLOR is set.
var (
	cAccent, cBold, cDim, cGreen, cRed, cInv, cReset = "\033[38;5;79m", "\033[1m", "\033[2m", "\033[32m", "\033[31m", "\033[7m", "\033[0m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		cAccent, cBold, cDim, cGreen, cRed, cInv, cReset = "", "", "", "", "", "", ""
	}
}

// cmdMenu is what `./trigger` with no arguments opens.
func cmdMenu() error {
	var c common
	if err := c.resolveRoot(); err != nil {
		return err
	}
	pages := [][]menuItem{
		pageSetup: {
			{"1", "PagerDuty settings", "Region (US/EU), API key and your email. Each one is checked against PagerDuty and saved.", "", false, func() error {
				confirmed = nil
				_, err := ensureSetup(c.root)
				return err
			}},
			{"2", "Check this Mac is ready", "Checks internet and your PagerDuty settings, downloads Terraform/kind/kubectl if needed, and checks Docker. Changes nothing.", "", false, func() error { return cmdDoctor() }},
			{"3", "Preview PagerDuty changes", "Shows what would be created, updated or deleted in PagerDuty. Changes nothing.", "", false, func() error { return cmdPlan(nil) }},
			{"4", "Create / update PagerDuty objects", "Creates the services, business service, orchestrations and workflows, showing you the list first. Run it again after installing a new version of Reinvent.", "", false, func() error { return cmdCreate(nil) }},
			{"5", "Start cluster + workloads (optional)", "Creates the local Kubernetes cluster and the demo workloads. Needs Docker. Only the remediation part of the story uses it.", "", false, func() error { return cmdCluster([]string{"up"}) }},
			{"6", "Open the setup guide", "The few steps done in the PagerDuty web app (Paige on the escalation policy, Kubernetes workflow steps, Slack, status page).", "", false, func() error { return openURL(guideURL) }},
			{"7", "Delete PagerDuty objects", "Resolves open demo alerts, then deletes everything Reinvent created in PagerDuty, after showing you the list.", "", false, func() error { return cmdDestroy(nil) }},
			{"8", "Delete local cluster", "Deletes the Kubernetes cluster and everything in it.", "This deletes the local cluster and everything in it. Sure?", false, func() error { return cmdCluster([]string{"down"}) }},
		},
		pageDemo: {
			{"1", "RUN THE DEMO", "Closes any previous run, then sends 2 change events and 11 alerts over about 30 seconds. Opens real incidents.", "Send the demo events to PagerDuty now?", true, func() error { return cmdRun(nil) }},
			{"2", "Resolve the demo", "Resolves every alert from demo runs that are still open. Do this when you've finished presenting.", "", false, func() error { return cmdResolve(nil) }},
			{"3", "Reset cluster for next demo", "Puts claims-reconciliation back without a memory limit, so the leak happens again. Only if you use the cluster.", "", false, func() error { return cmdCluster([]string{"reset"}) }},
			{"4", "Preview demo events", "Prints every event RUN THE DEMO would send. Sends nothing.", "", false, func() error { return cmdRun([]string{"-dry-run", "-no-wait"}) }},
			{"5", "Export payloads (curl/Postman)", "Writes every event as plain JSON with your real routing keys, plus send.sh and resolve.sh, to a \"Reinvent payloads\" folder on your Desktop.", "", false, func() error { return cmdExport([]string{"-keys", "-open"}) }},
			{"6", "Open the demo script", "The story, beat by beat: what to show and what to say.", "", false, func() error { return openURL(storyURL) }},
		},
	}

	tty, err := newRawTTY()
	if err != nil {
		return err
	}
	defer tty.restore()

	st := readStatus(c.root)
	page := pageSetup
	if st.setupDone() {
		page = pageDemo
	}
	sel, first := -1, true
	for {
		items := pages[page]
		next := st.next(page)
		if sel < 0 || sel >= len(items) {
			sel = max(indexOf(items, next), 0)
		}
		hdr := settled
		if first {
			hdr = headerFrame{}
		}
		draw(items, sel, st, page, next, hdr)
		if first {
			animateHeader()
			first = false
		}

		key := tty.readKey()
		switch key {
		case "up", "k":
			sel = (sel + len(items) - 1) % len(items)
			continue
		case "down", "j":
			sel = (sel + 1) % len(items)
			continue
		case "left", "right", "tab":
			page, sel = 1-page, -1
			continue
		case "q", "esc", "ctrl-d":
			tty.restore()
			fmt.Print("\033[H\033[2J")
			return nil
		case "enter":
		default:
			i := indexOf(items, key)
			if i < 0 {
				continue
			}
			sel = i
		}

		it := items[sel]
		tty.restore()
		fmt.Print("\033[H\033[2J")
		fmt.Printf("%s%s▸ %s%s\n%s%s%s\n\n", cAccent, cBold, it.label, cReset, cDim, it.help, cReset)
		run := true
		if it.confirm != "" {
			ok, err := askYesNo(it.confirm, it.confirmYes)
			run = err == nil && ok
		}
		wasDone := st.setupDone()
		if run {
			if err := it.run(); err != nil {
				fmt.Printf("\n%s✗ That didn't work:%s %v\n", cRed, cReset, err)
			} else {
				fmt.Printf("\n%s✓ Done.%s\n", cGreen, cReset)
				sel = -1 // jump to the next suggested step
			}
		} else {
			fmt.Println("Skipped.")
		}
		st = readStatus(c.root)
		if !wasDone && st.setupDone() {
			page, sel = pageDemo, -1
			fmt.Printf("\n%sSetup is complete. The menu now opens on the DEMO page.%s\n", cGreen, cReset)
		}
		ask(cDim + "Press Enter to go back to the menu… " + cReset)
		tty.raw()
	}
}

func draw(items []menuItem, sel int, st status, page int, next string, hdr headerFrame) {
	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	b.WriteString(header(hdr))

	dot := func(ok bool, yes, no string) string {
		if ok {
			return cGreen + "●" + cReset + " " + yes
		}
		return cDim + "○ " + no + cReset
	}
	fmt.Fprintf(&b, "  %-12s %s\n", "Settings", dot(st.ready, st.settings, "not set up"))
	switch {
	case st.created && st.outdated:
		fmt.Fprintf(&b, "  %-12s %s! needs update%s (SETUP → 4)\n", "PagerDuty", cRed, cReset)
	default:
		fmt.Fprintf(&b, "  %-12s %s\n", "PagerDuty", dot(st.created, "objects created", "nothing created yet"))
	}
	fmt.Fprintf(&b, "  %-12s %s\n", "Cluster", dot(st.cluster, "kind-"+kindName+" running", "not running (optional)"))
	demo := cDim + "○ nothing open" + cReset
	if st.demoOpen {
		demo = cAccent + "● a demo run is open" + cReset + cDim + " (resolve it when you're done)" + cReset
	}
	fmt.Fprintf(&b, "  %-12s %s\n\n", "Demo", demo)

	// Page tabs
	b.WriteString("  ")
	for i, name := range pageNames {
		if i == page {
			fmt.Fprintf(&b, "%s%s%s %s %s", cInv, cAccent, cBold, name, cReset)
		} else {
			fmt.Fprintf(&b, "%s %s %s", cDim, name, cReset)
		}
		b.WriteString("  ")
	}
	b.WriteString(cDim + "← → switch" + cReset + "\n\n")

	for i, it := range items {
		hint := "  "
		if it.key == next {
			hint = cAccent + "★ " + cReset
		}
		if page == pageSetup && it.key == "7" {
			b.WriteString("  " + cDim + strings.Repeat("─", 40) + cReset + "\n")
		}
		if i == sel {
			fmt.Fprintf(&b, "  %s%s %s  %-36s %s%s\n", cInv, cAccent, it.key, it.label, cReset, hint)
		} else {
			fmt.Fprintf(&b, "   %s%s%s  %-36s %s\n", cDim, it.key, cReset, it.label, hint)
		}
	}
	fmt.Fprintf(&b, "\n  %s%s%s\n", cDim, wrap(items[sel].help, 64, "  "), cReset)
	fmt.Fprintf(&b, "\n  %s↑↓ move · Enter run · number jumps · ← → SETUP/DEMO · q quit · ★ suggested%s", cDim, cReset)
	fmt.Print(b.String())
}

func wrap(s string, width int, indent string) string {
	var out, line []string
	n := 0
	for _, w := range strings.Fields(s) {
		if n+len(w) > width && n > 0 {
			out = append(out, strings.Join(line, " "))
			line, n = nil, 0
		}
		line = append(line, w)
		n += len(w) + 1
	}
	out = append(out, strings.Join(line, " "))
	return strings.Join(out, "\n"+indent)
}

func indexOf(items []menuItem, key string) int {
	for i, it := range items {
		if it.key == key {
			return i
		}
	}
	return -1
}

func termWidth() int {
	c := exec.Command("stty", "size")
	c.Stdin = os.Stdin
	out, err := c.Output()
	if err != nil {
		return 80
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return 80
	}
	w, _ := strconv.Atoi(f[1])
	return w
}

// rawTTY switches the terminal to unbuffered, no-echo input (Ctrl-C still
// works) and always puts it back, including on Ctrl-C.
type rawTTY struct{ saved string }

func newRawTTY() (*rawTTY, error) {
	c := exec.Command("stty", "-g")
	c.Stdin = os.Stdin
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("not a terminal: %w", err)
	}
	t := &rawTTY{saved: strings.TrimSpace(string(out))}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		t.restore()
		fmt.Println()
		os.Exit(130)
	}()
	t.raw()
	return t, nil
}

func (t *rawTTY) stty(args ...string) {
	c := exec.Command("stty", args...)
	c.Stdin = os.Stdin
	_ = c.Run()
}

func (t *rawTTY) raw() {
	t.stty("-icanon", "-echo", "min", "1")
	fmt.Print("\033[?25l") // hide cursor
}

func (t *rawTTY) restore() {
	t.stty(t.saved)
	fmt.Print("\033[?25h")
}

func (t *rawTTY) readKey() string {
	b, err := stdin.ReadByte()
	if err != nil {
		return "ctrl-d"
	}
	switch b {
	case '\r', '\n':
		return "enter"
	case '\t':
		return "tab"
	case 4:
		return "ctrl-d"
	case 27:
		if stdin.Buffered() == 0 {
			return "esc"
		}
		b2, _ := stdin.ReadByte()
		if b2 != '[' && b2 != 'O' {
			return ""
		}
		switch b3, _ := stdin.ReadByte(); b3 {
		case 'A':
			return "up"
		case 'B':
			return "down"
		case 'C':
			return "right"
		case 'D':
			return "left"
		}
		return ""
	}
	return strings.ToLower(string(b))
}

type status struct {
	settings string
	ready    bool
	created  bool
	outdated bool // applied by an older version: outputs this version needs are missing
	cluster  bool
	demoOpen bool
}

// setupDone: everything the demo needs is in place (the cluster is optional).
func (st status) setupDone() bool { return st.ready && st.created && !st.outdated }

// next is the suggested item (★) on a page.
func (st status) next(page int) string {
	if page == pageSetup {
		switch {
		case !st.ready:
			return "1"
		case !st.created || st.outdated:
			return "4"
		case !st.cluster:
			return "5"
		}
		return ""
	}
	if !st.setupDone() {
		return ""
	}
	if st.demoOpen {
		return "2"
	}
	return "1"
}

func readStatus(root string) status {
	var st status
	region, tok, email := os.Getenv(envRegion), os.Getenv(envToken), os.Getenv(envEmail)
	if confirmed != nil {
		region, tok, email = confirmed.region, confirmed.token, confirmed.email
	}
	st.ready = tok != "" && email != ""
	if region == "" {
		region = "us"
	}
	st.settings = fmt.Sprintf("%s · %s · key %s", strings.ToUpper(region), email, mask(tok))
	st.created, st.outdated = tfState(filepath.Join(root, "terraform", "terraform.tfstate"))
	st.cluster = kindExists()
	runs := openRuns(root)
	st.demoOpen = len(runs) > 0 && !(len(runs) == 1 && runs[0] == "")
	return st
}

// requiredOutputs are the Terraform outputs this version of the app reads.
// A state without them was applied by an older version and needs `create`.
var requiredOutputs = []string{"routing_key", "change_routing_keys"}

func tfState(path string) (created, outdated bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	var s struct {
		Resources []json.RawMessage          `json:"resources"`
		Outputs   map[string]json.RawMessage `json:"outputs"`
	}
	if json.Unmarshal(raw, &s) != nil || len(s.Resources) == 0 {
		return false, false
	}
	for _, o := range requiredOutputs {
		if _, ok := s.Outputs[o]; !ok {
			return true, true
		}
	}
	return true, false
}

// guideURL is the setup guide on GitHub (the manual, in-browser steps).
const guideURL = "https://github.com/" + githubRepo + "/tree/main/instructions"

// storyURL is the demo script.
const storyURL = "https://github.com/" + githubRepo + "/blob/main/story/README.md"

func openURL(u string) error {
	fmt.Println("Opening", u)
	return exec.Command("open", u).Run()
}
