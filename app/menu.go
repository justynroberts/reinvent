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
	run              func() error
}

const banner = `
 ██████╗ ███████╗██╗███╗   ██╗██╗   ██╗███████╗███╗   ██╗████████╗
 ██╔══██╗██╔════╝██║████╗  ██║██║   ██║██╔════╝████╗  ██║╚══██╔══╝
 ██████╔╝█████╗  ██║██╔██╗ ██║██║   ██║█████╗  ██╔██╗ ██║   ██║
 ██╔══██╗██╔══╝  ██║██║╚██╗██║╚██╗ ██╔╝██╔══╝  ██║╚██╗██║   ██║
 ██║  ██║███████╗██║██║ ╚████║ ╚████╔╝ ███████╗██║ ╚████║   ██║
 ╚═╝  ╚═╝╚══════╝╚═╝╚═╝  ╚═══╝  ╚═══╝  ╚══════╝╚═╝  ╚═══╝   ╚═╝`

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
	items := []menuItem{
		{"1", "PagerDuty settings", "Region (US/EU), API key and your email. Each one is checked against PagerDuty and saved to .env.", "", func() error {
			confirmed = nil
			_, err := ensureSetup(c.root)
			return err
		}},
		{"2", "Preview PagerDuty changes", "terraform plan. Read-only: shows what would be created or changed.", "", func() error { return cmdPlan(nil) }},
		{"3", "Create / update PagerDuty objects", "terraform apply: services, business service, EP, grouping, orchestrations and workflow shells. Also run this after installing a new version of Reinvent. Then do the manual steps in the setup guide.", "", func() error { return cmdCreate(nil) }},
		{"4", "Start cluster + workloads", "Creates the local kind cluster if needed (Docker must be running) and applies the demo workloads.", "", func() error { return cmdCluster([]string{"up"}) }},
		{"5", "RUN THE DEMO", "Sends the change event, then the alerts, with pauses between them. Opens a real incident.", "This sends real alerts and opens an incident. Go?", func() error { return cmdRun(nil) }},
		{"6", "Preview demo events", "Prints every event that RUN THE DEMO would send. Sends nothing.", "", func() error { return cmdRun([]string{"-dry-run", "-no-wait"}) }},
		{"7", "Resolve demo alerts", "Resolves every alert the demo opened.", "", func() error { return cmdResolve(nil) }},
		{"8", "Reset cluster for next demo", "Puts claims-reconciliation back without a memory limit, so the leak happens again.", "", func() error { return cmdCluster([]string{"reset"}) }},
		{"9", "Delete PagerDuty objects", "Resolves the alerts, then terraform destroy. Workflow steps you added by hand are lost.", "", func() error { return cmdDestroy(nil) }},
		{"e", "Export payloads (JSON for curl/Postman)", "Writes every event as plain JSON, with your real routing keys, plus send.sh and resolve.sh, to a \"Reinvent payloads\" folder on your Desktop, and opens it.", "", func() error { return cmdExport([]string{"-keys", "-open"}) }},
		{"c", "Check this Mac is ready", "Checks internet, your PagerDuty settings, downloads Terraform/kind/kubectl if needed, and checks Docker. Changes nothing.", "", func() error { return cmdDoctor() }},
		{"g", "Open the setup guide", "Opens the step-by-step guide for the parts done in the PagerDuty web app (Paige, workflow steps, Slack, status page).", "", func() error { return openURL(guideURL) }},
		{"0", "Delete local cluster", "Deletes the kind cluster and everything in it.", "This deletes the kind cluster and everything in it. Sure?", func() error { return cmdCluster([]string{"down"}) }},
	}

	tty, err := newRawTTY()
	if err != nil {
		return err
	}
	defer tty.restore()

	sel := -1
	stale := true
	var st status
	for {
		if stale { // shells out to kind/docker, so only after an action
			st, stale = readStatus(c.root), false
		}
		if sel < 0 {
			sel = indexOf(items, st.next)
		}
		draw(items, sel, st)

		key := tty.readKey()
		switch key {
		case "up", "k":
			sel = (sel + len(items) - 1) % len(items)
			continue
		case "down", "j", "tab":
			sel = (sel + 1) % len(items)
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
			ok, err := askYesNo(it.confirm, false)
			run = err == nil && ok
		}
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
		stale = true
		ask(cDim + "Press Enter to go back to the menu… " + cReset)
		tty.raw()
	}
}

func draw(items []menuItem, sel int, st status) {
	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	if termWidth() >= 68 {
		b.WriteString(cAccent + cBold + banner + cReset + "\n")
	} else {
		b.WriteString("\n " + cAccent + cBold + "R E I N V E N T" + cReset + "\n")
	}
	b.WriteString(" " + cDim + "claims-reconciliation · memory leak · PagerDuty demo · " + version + cReset + "\n\n")

	dot := func(ok bool, yes, no string) string {
		if ok {
			return cGreen + "●" + cReset + " " + yes
		}
		return cDim + "○ " + no + cReset
	}
	fmt.Fprintf(&b, "  %-12s %s\n", "Settings", dot(st.ready, st.settings, "not set up"))
	switch {
	case st.created && st.outdated:
		fmt.Fprintf(&b, "  %-12s %s! needs update%s (choose 3)\n", "PagerDuty", cRed, cReset)
	default:
		fmt.Fprintf(&b, "  %-12s %s\n", "PagerDuty", dot(st.created, "objects created", "nothing created yet"))
	}
	fmt.Fprintf(&b, "  %-12s %s\n\n", "Cluster", dot(st.cluster, "kind-"+kindName+" running", "not running"))

	for i, it := range items {
		hint := "  "
		if it.key == st.next {
			hint = cAccent + "★ " + cReset
		}
		if i == 4 || i == 8 || i == 10 {
			b.WriteString("  " + cDim + strings.Repeat("─", 40) + cReset + "\n")
		}
		if i == sel {
			fmt.Fprintf(&b, "  %s%s %s  %-34s %s%s\n", cInv, cAccent, it.key, it.label, cReset, hint)
		} else {
			fmt.Fprintf(&b, "   %s%s%s  %-34s %s\n", cDim, it.key, cReset, it.label, hint)
		}
	}
	fmt.Fprintf(&b, "\n  %s%s%s\n", cDim, wrap(items[sel].help, 64, "  "), cReset)
	fmt.Fprintf(&b, "\n  %s↑↓ move · Enter run · 0-9/c/e/g jump · q quit · ★ suggested next step%s", cDim, cReset)
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
	next     string
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

	switch {
	case !st.ready:
		st.next = "1"
	case !st.created || st.outdated:
		st.next = "3"
	case !st.cluster:
		st.next = "4"
	default:
		st.next = "5"
	}
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

func openURL(u string) error {
	fmt.Println("Opening", u)
	return exec.Command("open", u).Run()
}
