package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotEnvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("# keep me\nPAGERDUTY_SERVICE_REGION=eu\nPAGERDUTY_TOKEN=old\n"), 0o644)

	if err := saveDotEnv(dir, "PAGERDUTY_TOKEN", "new"); err != nil {
		t.Fatal(err)
	}
	if err := saveDotEnv(dir, "PD_ROUTING_KEY", "rk"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	got := string(raw)
	for _, want := range []string{"# keep me\n", "PAGERDUTY_SERVICE_REGION=eu\n", "PAGERDUTY_TOKEN=new\n", "PD_ROUTING_KEY=rk\n"} {
		if !strings.Contains(got, want) {
			t.Errorf(".env missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "old") {
		t.Errorf("old token not replaced:\n%s", got)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}

	t.Setenv("PAGERDUTY_TOKEN", "from-real-env") // real environment wins
	os.Unsetenv("PD_ROUTING_KEY")
	t.Cleanup(func() { os.Unsetenv("PD_ROUTING_KEY") })
	if err := loadDotEnv(dir); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("PAGERDUTY_TOKEN"); v != "from-real-env" {
		t.Errorf("PAGERDUTY_TOKEN = %q, want real env to win", v)
	}
	if v := os.Getenv("PD_ROUTING_KEY"); v != "rk" {
		t.Errorf("PD_ROUTING_KEY = %q, want rk from .env", v)
	}
}

func TestParseEnvLine(t *testing.T) {
	cases := map[string][2]string{
		`export A="x y"`: {"A", "x y"},
		`B='z'`:          {"B", "z"},
		` C = d `:        {"C", "d"},
	}
	for in, want := range cases {
		k, v, ok := parseEnvLine(in)
		if !ok || k != want[0] || v != want[1] {
			t.Errorf("%q -> %q %q %v", in, k, v, ok)
		}
	}
	for _, in := range []string{"", "# c", "novalue"} {
		if _, _, ok := parseEnvLine(in); ok {
			t.Errorf("%q should be skipped", in)
		}
	}
}
