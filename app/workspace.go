package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	reinvent "github.com/fintonlabs/reinvent"
)

// version is set at release time with -ldflags "-X main.version=1.2.3".
var version = "dev"

// workspaceDir is where the distributed app keeps its files and state:
// .env, Terraform state, and the extracted payloads / Terraform / k8s files.
func workspaceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Reinvent"), nil
}

// prepareWorkspace extracts the embedded files into dir when the app version
// changed (always for dev builds). Your settings (.env), Terraform state and
// downloaded tools are never touched.
func prepareWorkspace(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	stamp := filepath.Join(dir, ".app-version")
	if old, err := os.ReadFile(stamp); err == nil && string(old) == version && version != "dev" {
		return nil
	}
	err := fs.WalkDir(reinvent.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := reinvent.Files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("unpacking app files into %s: %w", dir, err)
	}
	return os.WriteFile(stamp, []byte(version), 0o644)
}

// useWorkspace switches to the workspace when no repo checkout is found.
func useWorkspace() (string, error) {
	dir, err := workspaceDir()
	if err != nil {
		return "", err
	}
	if err := prepareWorkspace(dir); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, "payloads", "scenario.json")); err != nil {
		return "", errors.New("app files are missing from " + dir)
	}
	return dir, nil
}

// githubRepo is where releases, the installer and the setup guide live.
const githubRepo = "justynroberts/reinvent"
