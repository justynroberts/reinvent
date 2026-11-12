package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormRegion(t *testing.T) {
	for in, want := range map[string]string{"US": "us", " eu ": "eu", "E": "eu", "usa": "us", "": "", "apac": ""} {
		if got := normRegion(in); got != want {
			t.Errorf("normRegion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTfvarsEmail(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "terraform"), 0o755)
	path := filepath.Join(dir, "terraform", "terraform.tfvars")
	if got := tfvarsEmail(dir); got != "" {
		t.Errorf("no file: got %q", got)
	}
	os.WriteFile(path, []byte(`responder_email = "you@example.com"`+"\n"), 0o644)
	if got := tfvarsEmail(dir); got != "" {
		t.Errorf("placeholder should be ignored, got %q", got)
	}
	os.WriteFile(path, []byte("# x\n  responder_email = \"a@b.co\"\n"), 0o644)
	if got := tfvarsEmail(dir); got != "a@b.co" {
		t.Errorf("got %q, want a@b.co", got)
	}
}

func TestMask(t *testing.T) {
	if got := mask("u+abcdefWXYZ"); got != "****WXYZ" {
		t.Errorf("mask = %q", got)
	}
	if got := mask("abc"); got != "****" {
		t.Errorf("short mask = %q", got)
	}
}

func TestRunTracking(t *testing.T) {
	dir := t.TempDir()
	if got := openRuns(dir); len(got) != 1 || got[0] != "" {
		t.Errorf("untracked copy should resolve the old fixed keys, got %q", got)
	}
	if err := saveRuns(dir, []string{"run-a", "run-b"}); err != nil {
		t.Fatal(err)
	}
	if got := openRuns(dir); len(got) != 2 || got[1] != "run-b" {
		t.Errorf("openRuns = %q", got)
	}
	saveRuns(dir, nil)
	if got := openRuns(dir); len(got) != 0 {
		t.Errorf("after resolve, openRuns = %q, want none", got)
	}
	if runKey("reinvent/x", "run-a") != "reinvent/x/run-a" || runKey("reinvent/x", "") != "reinvent/x" {
		t.Error("runKey suffix wrong")
	}
}
