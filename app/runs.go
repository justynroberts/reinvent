package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Every demo run gets its own ID, appended to each alert's dedup_key.
// Without it, a second run while the first is still open is merged into the
// open alerts by PagerDuty and no new incidents appear.
//
// Open runs are listed in <root>/.demo-runs so "Resolve" closes exactly
// those. A missing file means this copy never tracked runs: resolve then
// also covers the old fixed keys (run ID "").

func runsPath(root string) string { return filepath.Join(root, ".demo-runs") }

func newRunID() string { return "run-" + strconv.FormatInt(time.Now().Unix(), 36) }

// openRuns returns the runs that may still have open alerts.
func openRuns(root string) []string {
	raw, err := os.ReadFile(runsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return []string{""} // never tracked: old fixed keys may be open
	}
	var ids []string
	_ = json.Unmarshal(raw, &ids)
	return ids
}

func saveRuns(root string, ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	raw, _ := json.Marshal(ids)
	return os.WriteFile(runsPath(root), raw, 0o644)
}

// runKey is the dedup_key for an alert in a given run.
func runKey(base, runID string) string {
	if runID == "" {
		return base
	}
	return base + "/" + runID
}

// resolveOpenRuns resolves every alert of every open run, then forgets them.
func resolveOpenRuns(root string, sc *Scenario, s *sender) error {
	runs := openRuns(root)
	if len(runs) == 0 {
		fmt.Println("No open demo runs to resolve.")
		return nil
	}
	for _, id := range runs {
		label := id
		if id == "" {
			label = "earlier runs (before run tracking)"
		}
		fmt.Printf("Resolving %s…\n", label)
		if err := sc.Resolve(s, id); err != nil {
			return err
		}
	}
	if s.dryRun {
		return nil
	}
	return saveRuns(root, nil)
}
