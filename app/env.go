package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// loadDotEnv reads <root>/.env into the process environment. Variables
// already set in the real environment win. Missing file is not an error.
func loadDotEnv(root string) error {
	f, err := os.Open(filepath.Join(root, ".env"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := parseEnvLine(sc.Text())
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return sc.Err()
}

func parseEnvLine(line string) (key, val string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	key, val, ok = strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key, val = strings.TrimSpace(key), strings.TrimSpace(val)
	if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
		val = val[1 : len(val)-1]
	}
	return key, val, key != ""
}

// saveDotEnv sets key=val in <root>/.env, replacing an existing line for key
// and keeping everything else. The file is written owner-only (0600).
func saveDotEnv(root, key, val string) error {
	path := filepath.Join(root, ".env")
	var lines []string
	if raw, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	replaced := false
	for i, l := range lines {
		if k, _, ok := parseEnvLine(l); ok && k == key {
			lines[i] = key + "=" + val
			replaced = true
		}
	}
	if !replaced {
		if len(lines) == 0 {
			lines = append(lines, "# Local secrets for the reinvent demo. Gitignored. Never commit.")
		}
		lines = append(lines, key+"="+val)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600) // WriteFile keeps old perms on an existing file
}

// isTerminal reports whether stdin is a TTY. A ModeCharDevice check is not
// enough (/dev/null is a char device); stty fails on anything but a TTY.
func isTerminal() bool {
	c := exec.Command("stty", "-g")
	c.Stdin = os.Stdin
	return c.Run() == nil
}

// readSecret reads a line with terminal echo off (via stty, to stay stdlib-only).
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	stty := func(arg string) {
		c := exec.Command("stty", arg)
		c.Stdin = os.Stdin
		_ = c.Run()
	}
	stty("-echo")
	defer func() { stty("echo"); fmt.Println() }()
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
