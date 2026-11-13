// Command trigger drives the claims-reconciliation demo. Run it via the
// ./trigger script at the repo root, which builds it first. With no arguments
// it opens a menu covering every step; the subcommands below are for scripts.
//
// Payloads live in ../payloads and are Go templates; edit them, not this code.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const usage = `usage: ./trigger              (opens the menu: start here)
       ./trigger <command> [flags]

commands:
  setup     check or change region, API key and responder email
  doctor    check this Mac: internet, PagerDuty settings, Terraform
  export    write every event as plain JSON + send.sh/resolve.sh (-keys for real keys)
  plan      terraform init + plan (read-only)
  create    terraform init + apply in terraform/ (prompts unless -auto-approve)
  run       send the scenario in payloads/scenario.json to PagerDuty
  resolve   send resolve events for every alert in the scenario
  destroy   resolve open alerts, then terraform destroy (prompts unless -auto-approve)

flags (after the command):
  -root DIR          repo root (default: found by walking up from the working dir)

environment (also read from <root>/.env; the real environment wins):
  PAGERDUTY_TOKEN            REST API key for plan/create/destroy
  PAGERDUTY_SERVICE_REGION   "us" or "eu"
  PD_RESPONDER_EMAIL         your PagerDuty login email (human responder on the EP)
  plan/create/destroy ask for these in a terminal, confirm saved values, check
  them against PagerDuty, and save them to .env.
  PD_ROUTING_KEY             overrides the alert routing key (global orchestration)
  PD_CHANGE_ROUTING_KEY      overrides the change-event routing key (every service)

run "trigger <command> -h" for command flags.
`

func main() {
	if len(os.Args) < 2 {
		if !isTerminal() && relaunchInTerminal() {
			return
		}
		if !isTerminal() {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		if err := cmdMenu(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "setup":
		err = cmdSetup(args)
	case "doctor":
		err = cmdDoctor()
	case "export":
		err = cmdExport(args)
	case "version", "-v", "--version":
		fmt.Println("reinvent", version)
	case "plan":
		err = cmdPlan(args)
	case "create":
		err = cmdCreate(args)
	case "run":
		err = cmdRun(args)
	case "resolve":
		err = cmdResolve(args)
	case "destroy":
		err = cmdDestroy(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// common holds flags shared by every command.
type common struct {
	root       string
	scenario   string
	routingKey string
}

func (c *common) register(fs *flag.FlagSet, withScenario bool) {
	fs.StringVar(&c.root, "root", "", "repo root (default: auto-detect)")
	if withScenario {
		fs.StringVar(&c.scenario, "scenario", "", "scenario file (default: <root>/payloads/scenario.json)")
		fs.StringVar(&c.routingKey, "routing-key", "", "alert routing key, global orchestration (default: $PD_ROUTING_KEY, then terraform output)")
	}
}

func (c *common) resolveRoot() error {
	if err := c.findRoot(); err != nil {
		return err
	}
	if err := loadDotEnv(c.root); err != nil {
		return fmt.Errorf("reading .env: %w", err)
	}
	return nil
}

func (c *common) findRoot() error {
	if c.root != "" {
		abs, err := filepath.Abs(c.root)
		c.root = abs
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "payloads", "scenario.json")); err == nil {
			c.root = dir
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Not inside a checkout: the downloaded app. Use its own workspace.
			ws, err := useWorkspace()
			c.root = ws
			return err
		}
		dir = parent
	}
}

func (c *common) scenarioPath() string {
	if c.scenario != "" {
		return c.scenario
	}
	return filepath.Join(c.root, "payloads", "scenario.json")
}

func (c *common) tfDir() string { return filepath.Join(c.root, "terraform") }

// changeKeys returns the change-event routing key per service name, from
// terraform output. PD_CHANGE_ROUTING_KEY, if set, is used for every service.
func (c *common) changeKeys() (map[string]string, error) {
	if k := os.Getenv("PD_CHANGE_ROUTING_KEY"); k != "" {
		return map[string]string{"*": k}, nil
	}
	raw, err := terraformOutput(c.tfDir(), "change_routing_keys")
	if err != nil {
		return nil, c.notReady(err)
	}
	keys := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil, fmt.Errorf("reading change_routing_keys: %w", err)
	}
	return keys, nil
}

// key returns the alert routing key (global orchestration): flag, then env, then terraform output.
func (c *common) key() (string, error) {
	if c.routingKey != "" {
		return c.routingKey, nil
	}
	if k := os.Getenv("PD_ROUTING_KEY"); k != "" {
		return k, nil
	}
	k, err := terraformOutput(c.tfDir(), "routing_key")
	if err != nil {
		return "", c.notReady(err)
	}
	return k, nil
}

func cmdCreate(args []string) error {
	var c common
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	c.register(fs, false)
	auto := fs.Bool("auto-approve", false, "apply without asking")
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	set, err := ensureSetup(c.root)
	if err != nil {
		return err
	}
	vars, err := preflight(c.root, set, true)
	if err != nil {
		return err
	}
	if err := planAndApply(c.tfDir(), vars, false, *auto, false); err != nil {
		return err
	}
	fmt.Println("\nNext: the few steps done in the PagerDuty web app (SETUP → 5 \"Open the setup guide\"), then DEMO → 1 RUN THE DEMO.")
	return nil
}

func cmdRun(args []string) error {
	var c common
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	c.register(fs, true)
	dry := fs.Bool("dry-run", false, "render and print payloads without sending")
	fast := fs.Bool("no-wait", false, "ignore wait_after between steps")
	only := fs.String("step", "", "send only steps whose name contains this text")
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	sc, err := loadScenario(c.scenarioPath())
	if err != nil {
		return err
	}
	key, ckeys := "DRY-RUN", map[string]string{"*": "DRY-RUN"}
	if *dry {
		return sc.Run(newSender(key, ckeys, true), runOptions{noWait: *fast, only: *only})
	}
	if key, err = c.key(); err != nil {
		return err
	}
	if ckeys, err = c.changeKeys(); err != nil {
		return err
	}
	s := newSender(key, ckeys, false)
	// Start clean: close the previous run so this one opens fresh incidents.
	if prev := openRuns(c.root); len(prev) > 0 {
		fmt.Println("Closing the previous demo run first, so this one opens fresh incidents.")
		if err := resolveOpenRuns(c.root, sc, s); err != nil {
			return err
		}
		fmt.Println("Waiting 5s for PagerDuty to close them…")
		time.Sleep(5 * time.Second)
		fmt.Println()
	}
	id := newRunID()
	if err := saveRuns(c.root, append(openRuns(c.root), id)); err != nil {
		return err
	}
	fmt.Printf("Demo run %s\n", id)
	return sc.Run(s, runOptions{noWait: *fast, only: *only, runID: id})
}

func cmdResolve(args []string) error {
	var c common
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	c.register(fs, true)
	dry := fs.Bool("dry-run", false, "print resolve events without sending")
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	sc, err := loadScenario(c.scenarioPath())
	if err != nil {
		return err
	}
	key := "DRY-RUN"
	if !*dry {
		if key, err = c.key(); err != nil {
			return err
		}
	}
	return resolveOpenRuns(c.root, sc, newSender(key, nil, *dry))
}

func cmdDestroy(args []string) error {
	var c common
	fs := flag.NewFlagSet("destroy", flag.ExitOnError)
	c.register(fs, true)
	auto := fs.Bool("auto-approve", false, "delete without asking")
	skipResolve := fs.Bool("skip-resolve", false, "do not resolve scenario alerts first")
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	set, err := ensureSetup(c.root)
	if err != nil {
		return err
	}
	if !*skipResolve {
		if key, err := c.key(); err != nil {
			fmt.Fprintln(os.Stderr, "warning: skipping resolve:", err)
		} else if sc, err := loadScenario(c.scenarioPath()); err != nil {
			fmt.Fprintln(os.Stderr, "warning: skipping resolve:", err)
		} else if err := resolveOpenRuns(c.root, sc, newSender(key, nil, false)); err != nil {
			fmt.Fprintln(os.Stderr, "warning: resolve failed, continuing:", err)
		}
	}
	vars, err := preflight(c.root, set, false)
	if err != nil {
		return err
	}
	return planAndApply(c.tfDir(), vars, true, *auto, false)
}

func cmdPlan(args []string) error {
	var c common
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	c.register(fs, false)
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	set, err := ensureSetup(c.root)
	if err != nil {
		return err
	}
	vars, err := preflight(c.root, set, true)
	if err != nil {
		return err
	}
	return planAndApply(c.tfDir(), vars, false, false, true)
}

func cmdSetup(args []string) error {
	var c common
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	c.register(fs, false)
	fs.Parse(args)
	if err := c.resolveRoot(); err != nil {
		return err
	}
	_, err := ensureSetup(c.root)
	return err
}

// notReady explains a missing routing key in plain words: nothing created
// yet, or created by an older version and not updated since.
func (c *common) notReady(cause error) error {
	created, outdated := tfState(filepath.Join(c.tfDir(), "terraform.tfstate"))
	switch {
	case !created:
		return errors.New("the PagerDuty objects haven't been created yet. In the menu, go to SETUP → 4 \"Create / update PagerDuty objects\" first")
	case outdated:
		return errors.New("your PagerDuty setup is from an older version of Reinvent. In the menu, go to SETUP → 4 \"Create / update PagerDuty objects\" to update it, then run the demo again")
	}
	return fmt.Errorf("couldn't read the routing key from Terraform: %w", cause)
}
