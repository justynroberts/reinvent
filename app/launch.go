package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// relaunchInTerminal handles a double-click on Reinvent.app: Finder starts
// the binary with no terminal, so open a Terminal window that runs it.
// Returns false when not running from an app bundle.
func relaunchInTerminal() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return false
	}
	if !strings.Contains(exe, ".app/Contents/MacOS/") {
		return false
	}
	return exec.Command("open", "-a", "Terminal", exe).Run() == nil
}
