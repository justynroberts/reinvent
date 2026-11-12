package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	kindName       = "reinvent"
	defaultContext = "kind-" + kindName
)

// cmdCluster manages the local kind cluster and the demo workloads.
// Every kubectl call names its context, so it never touches another cluster.
func cmdCluster(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: trigger cluster up|reset|down [-context NAME]")
	}
	sub := args[0]
	var c common
	fs := flag.NewFlagSet("cluster "+sub, flag.ExitOnError)
	c.register(fs, false)
	kctx := fs.String("context", defaultContext, "kubectl context to use")
	fs.Parse(args[1:])
	if err := c.resolveRoot(); err != nil {
		return err
	}
	k8s := func(f string) string { return filepath.Join(c.root, "k8s", f) }
	usingKind := *kctx == defaultContext

	switch sub {
	case "up":
		if usingKind {
			if err := needKind(); err != nil {
				return err
			}
			if kindExists() {
				fmt.Printf("kind cluster %q already exists, reusing it.\n", kindName)
			} else if err := sh("kind", "create", "cluster", "--config", k8s("kind-config.yaml")); err != nil {
				return err
			}
		}
		if err := sh("kubectl", "--context", *kctx, "apply", "-f", k8s("healthcare.yaml"), "-f", k8s("remediation-toolbox.yaml")); err != nil {
			return err
		}
		fmt.Println("\nWorkloads applied. Pods take a minute to start:")
		return sh("kubectl", "--context", *kctx, "-n", "healthcare", "get", "pods", "-o", "wide")

	case "reset":
		fmt.Println("Recreating claims-reconciliation without a memory limit (the bug), for the next demo.")
		if err := sh("kubectl", "--context", *kctx, "-n", "healthcare", "delete",
			"deployment/claims-reconciliation", "pod/claims-reconciliation", "--ignore-not-found"); err != nil {
			return err
		}
		return sh("kubectl", "--context", *kctx, "apply", "-f", k8s("healthcare.yaml"))

	case "down":
		if !usingKind {
			return errors.New("cluster down only deletes the local kind cluster; it won't touch " + *kctx)
		}
		if err := needKind(); err != nil {
			return err
		}
		if !kindExists() {
			fmt.Printf("kind cluster %q doesn't exist, nothing to delete.\n", kindName)
			return nil
		}
		return sh("kind", "delete", "cluster", "--name", kindName)
	}
	return fmt.Errorf("unknown cluster command %q (want up, reset or down)", sub)
}

func needKind() error {
	docker, err := tool("docker")
	if err != nil {
		return err
	}
	if exec.Command(docker, "info").Run() != nil {
		return errors.New("Docker isn't running. Open Docker Desktop (or Rancher Desktop / OrbStack), wait until it says it's running, then try again")
	}
	return nil
}

func kindExists() bool {
	kind := findTool("kind")
	if kind == "" {
		return false
	}
	out, err := exec.Command(kind, "get", "clusters").Output()
	if err != nil {
		return false
	}
	for _, l := range strings.Fields(string(out)) {
		if l == kindName {
			return true
		}
	}
	return false
}

// sh runs a tool (downloading it on first use) wired to this terminal, echoing it first.
func sh(name string, args ...string) error {
	path, err := tool(name)
	if err != nil {
		return err
	}
	fmt.Printf("$ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, args[0], err)
	}
	return nil
}
