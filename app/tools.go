package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Pinned tool versions. The app downloads these on first use, checks their
// SHA-256 against the publisher's checksum file, and keeps them in the
// workspace, so users never install Terraform, kind or kubectl themselves.
const (
	terraformVersion = "1.9.8"
	kindVersion      = "v0.24.0"
	kubectlVersion   = "v1.31.1"
)

type toolSource struct {
	url, sumURL string
	inZip       string // name inside a zip; "" means the download is the binary
}

func toolSources(name string) (toolSource, bool) {
	arch := runtime.GOARCH // arm64 or amd64, same names all three publishers use
	switch name {
	case "terraform":
		file := fmt.Sprintf("terraform_%s_darwin_%s.zip", terraformVersion, arch)
		base := "https://releases.hashicorp.com/terraform/" + terraformVersion + "/"
		return toolSource{base + file, base + "terraform_" + terraformVersion + "_SHA256SUMS", "terraform"}, true
	case "kind":
		u := fmt.Sprintf("https://github.com/kubernetes-sigs/kind/releases/download/%s/kind-darwin-%s", kindVersion, arch)
		return toolSource{u, u + ".sha256sum", ""}, true
	case "kubectl":
		u := fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/%s/kubectl", kubectlVersion, arch)
		return toolSource{u, u + ".sha256", ""}, true
	}
	return toolSource{}, false
}

var resolved = map[string]string{}

func toolsDir() (string, error) {
	ws, err := workspaceDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(ws, "tools"), nil
}

// findTool returns a tool that is already present (PATH or downloaded), without downloading.
func findTool(name string) string {
	if p, ok := resolved[name]; ok {
		return p
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if dir, err := toolsDir(); err == nil {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// tool returns the path to name, downloading the pinned version if needed.
func tool(name string) (string, error) {
	if p := findTool(name); p != "" {
		resolved[name] = p
		return p, nil
	}
	if name == "docker" {
		if p := dockerOutsidePath(); p != "" {
			// kind calls `docker` itself, so make sure it can find it.
			os.Setenv("PATH", filepath.Dir(p)+string(os.PathListSeparator)+os.Getenv("PATH"))
			resolved[name] = p
			return p, nil
		}
		return "", errors.New("Docker isn't installed. Install Docker Desktop (free): https://www.docker.com/products/docker-desktop/ — Rancher Desktop or OrbStack also work. Only the cluster steps need it")
	}
	src, ok := toolSources(name)
	if !ok {
		return "", fmt.Errorf("%s is not installed", name)
	}
	dir, err := toolsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	fmt.Printf("First use: downloading %s (one-off)...\n", name)
	data, err := fetch(src.url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w (check your internet connection)", name, err)
	}
	if err := checkSum(data, src); err != nil {
		return "", fmt.Errorf("%s download failed its checksum, not using it: %w", name, err)
	}
	if src.inZip != "" {
		if data, err = unzipOne(data, src.inZip); err != nil {
			return "", err
		}
	}
	p := filepath.Join(dir, name)
	tmp := p + ".part"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", err
	}
	fmt.Printf("  %s ready.\n", name)
	resolved[name] = p
	return p, nil
}

func fetch(url string) ([]byte, error) {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// checkSum verifies data against the publisher's checksum file. The file is
// either a bare hash, "<hash>  <name>", or a list of such lines.
func checkSum(data []byte, src toolSource) error {
	sums, err := fetch(src.sumURL)
	if err != nil {
		return err
	}
	want := ""
	base := src.url[strings.LastIndex(src.url, "/")+1:]
	for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
		f := strings.Fields(line)
		if len(f) == 1 || (len(f) >= 2 && strings.TrimPrefix(f[1], "*") == base) {
			want = f[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s in %s", base, src.sumURL)
	}
	h := sha256.Sum256(data)
	if got := hex.EncodeToString(h[:]); !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 %s, expected %s", got, want)
	}
	return nil
}

func unzipOne(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in the download", name)
}

// dockerOutsidePath finds Docker Desktop's CLI where it lives when the
// installer didn't link it onto PATH.
func dockerOutsidePath() string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
		filepath.Join(home, ".docker", "bin", "docker"),
		"/Applications/Docker.app/Contents/Resources/bin/docker",
		filepath.Join(home, ".rd", "bin", "docker"),       // Rancher Desktop
		filepath.Join(home, ".orbstack", "bin", "docker"), // OrbStack
	} {
		if fi, err := os.Stat(p); err == nil && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}
