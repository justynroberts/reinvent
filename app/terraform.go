package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// terraformOutput reads one raw output value (works for sensitive outputs).
func terraformOutput(dir, name string) (string, error) {
	var out, errb bytes.Buffer
	tf, err := tool("terraform")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(tf, "-chdir="+dir, "output", "-raw", name)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("terraform output %s: %s", name, strings.TrimSpace(errb.String()))
	}
	v := strings.TrimSpace(out.String())
	if v == "" {
		return "", fmt.Errorf("terraform output %s is empty", name)
	}
	return v, nil
}
