package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Settings the setup wizard owns. All are saved to <root>/.env.
const (
	envToken  = "PAGERDUTY_TOKEN"
	envRegion = "PAGERDUTY_SERVICE_REGION"
	envEmail  = "PD_RESPONDER_EMAIL"
)

// settings is what plan/create/destroy need before Terraform runs.
type settings struct {
	region string // "us" or "eu"
	token  string
	email  string
}

// confirmed caches the answers so the menu asks once per session.
var confirmed *settings

// stdin is shared so successive prompts never lose buffered input.
var stdin = bufio.NewReader(os.Stdin)

// ensureSetup resolves region, API key and responder email. On a terminal it
// asks for anything missing and confirms anything already saved; every value
// is checked against PagerDuty before it is used. Without a terminal it uses
// what is saved and fails with a clear message if something is missing.
func ensureSetup(root string) (settings, error) {
	if confirmed != nil { // already confirmed in this session (menu)
		return *confirmed, nil
	}
	if _, err := tool("terraform"); err != nil {
		return settings{}, err
	}
	interactive := isTerminal()
	s := settings{
		region: normRegion(os.Getenv(envRegion)),
		token:  os.Getenv(envToken),
		email:  os.Getenv(envEmail),
	}
	if s.email == "" {
		s.email = tfvarsEmail(root)
	}

	if !interactive {
		if s.token == "" {
			return s, errors.New(envToken + " is not set and stdin is not a terminal; open Reinvent and choose \"PagerDuty settings\"")
		}
		if s.email == "" {
			return s, errors.New(envEmail + " is not set and stdin is not a terminal; open Reinvent and choose \"PagerDuty settings\"")
		}
		if s.region == "" {
			s.region = "us"
		}
		if err := checkToken(s.token, s.region); err != nil {
			return s, fmt.Errorf("API key check failed for region %s: %w", strings.ToUpper(s.region), err)
		}
		if _, err := findUser(s.token, s.region, s.email); err != nil {
			return s, err
		}
		applyRegion(s.region)
		return s, nil
	}

	fmt.Println("── PagerDuty setup ─────────────────────────────────────────")
	fmt.Println("Press Enter to keep the value shown in [brackets].")
	fmt.Println()

	for {
		if err := askRegion(root, &s); err != nil {
			return s, err
		}
		retryRegion, err := askToken(root, &s)
		if err != nil {
			return s, err
		}
		if retryRegion {
			continue
		}
		break
	}
	if err := askEmail(root, &s); err != nil {
		return s, err
	}

	fmt.Println()
	fmt.Println("── Using ───────────────────────────────────────────────────")
	fmt.Printf("  Region:           %s\n", strings.ToUpper(s.region))
	fmt.Printf("  API key:          %s\n", mask(s.token))
	fmt.Printf("  Responder email:  %s\n", s.email)
	fmt.Printf("  Saved in:         %s (mode 600, gitignored)\n", filepath.Join(root, ".env"))
	ok, err := askYesNo("Continue?", true)
	if err != nil {
		return s, err
	}
	if !ok {
		return s, errors.New("cancelled; nothing was changed in PagerDuty")
	}
	fmt.Println()
	applyRegion(s.region)
	confirmed = &s
	return s, nil
}

func askRegion(root string, s *settings) error {
	def := s.region
	if def == "" {
		def = "us"
	}
	for {
		ans, err := ask(fmt.Sprintf("Account region, US or EU (EU if your URL is *.eu.pagerduty.com) [%s]: ", strings.ToUpper(def)))
		if err != nil {
			return err
		}
		if ans == "" {
			ans = def
		}
		r := normRegion(ans)
		if r == "" {
			fmt.Println("  Type US or EU.")
			continue
		}
		s.region = r
		return saveDotEnv(root, envRegion, r)
	}
}

// askToken returns retryRegion=true when the user wants to pick the region again.
func askToken(root string, s *settings) (retryRegion bool, err error) {
	if s.token != "" {
		keep, err := askYesNo(fmt.Sprintf("API key %s is saved. Keep it?", mask(s.token)), true)
		if err != nil {
			return false, err
		}
		if !keep {
			s.token = ""
		}
	}
	if s.token == "" {
		fmt.Println("  Use an account REST API key with read/write access:")
		fmt.Println("  PagerDuty → Integrations → API Access Keys → Create New API Key.")
	}
	for attempt := 0; attempt < 3; attempt++ {
		if s.token == "" {
			tok, err := readSecret("PagerDuty API key (input hidden): ")
			if err != nil {
				return false, err
			}
			if tok == "" {
				fmt.Println("  Empty key, try again.")
				continue
			}
			s.token = tok
		}
		switch err := checkToken(s.token, s.region); {
		case errors.Is(err, errBadToken):
			fmt.Printf("  PagerDuty (%s) rejected that key.\n", strings.ToUpper(s.region))
			other := map[string]string{"us": "EU", "eu": "US"}[s.region]
			again, aerr := askYesNo(fmt.Sprintf("  Is the account actually %s? Switch region and retry?", other), false)
			if aerr != nil {
				return false, aerr
			}
			if again {
				s.region = normRegion(other)
				if err := saveDotEnv(root, envRegion, s.region); err != nil {
					return false, err
				}
				return true, nil // re-run the region/key loop with the same key
			}
			s.token = ""
			continue
		case err != nil:
			fmt.Fprintln(os.Stderr, "  warning: could not verify the key (network?), using it anyway:", err)
		default:
			fmt.Println("  Key verified.")
		}
		os.Setenv(envToken, s.token) // child terraform inherits it
		return false, saveDotEnv(root, envToken, s.token)
	}
	return false, errors.New("no valid API key entered")
}

func askEmail(root string, s *settings) error {
	for attempt := 0; attempt < 5; attempt++ {
		prompt := "Your PagerDuty login email (the human responder on the escalation policy): "
		if s.email != "" {
			prompt = fmt.Sprintf("Your PagerDuty login email (the human responder) [%s]: ", s.email)
		}
		ans, err := ask(prompt)
		if err != nil {
			return err
		}
		if ans != "" {
			s.email = ans
		}
		if s.email == "" {
			fmt.Println("  An email is required.")
			continue
		}
		if !emailRe.MatchString(s.email) {
			fmt.Printf("  %q doesn't look like an email address.\n", s.email)
			s.email = ""
			continue
		}
		name, err := findUser(s.token, s.region, s.email)
		switch {
		case errors.Is(err, errNoUser):
			fmt.Printf("  No user with email %s exists on this PagerDuty account.\n", s.email)
			fmt.Println("  Use the email you log in to PagerDuty with.")
			s.email = ""
			continue
		case err != nil:
			fmt.Fprintln(os.Stderr, "  warning: could not look the user up, using it anyway:", err)
		default:
			fmt.Printf("  Found user: %s.\n", name)
		}
		os.Setenv(envEmail, s.email)
		return saveDotEnv(root, envEmail, s.email)
	}
	return errors.New("no valid responder email entered")
}

var (
	errBadToken = errors.New("invalid API key (401)")
	errNoUser   = errors.New("user not found")
	emailRe     = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	tfvarsRe    = regexp.MustCompile(`(?m)^\s*responder_email\s*=\s*"([^"]*)"`)
)

func apiBase(region string) string {
	if region == "eu" {
		return "https://api.eu.pagerduty.com"
	}
	return "https://api.pagerduty.com"
}

func apiGet(token, region, path string, out any) (int, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase(region)+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Token token="+token)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, errBadToken
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("GET %s returned %s", path, resp.Status)
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

// checkToken makes one read-only call (GET /abilities), which works for both
// account and user API keys.
func checkToken(token, region string) error {
	_, err := apiGet(token, region, "/abilities", nil)
	return err
}

// findUser returns the display name of the user with exactly this email.
func findUser(token, region, email string) (string, error) {
	var body struct {
		Users []struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"users"`
	}
	if _, err := apiGet(token, region, "/users?limit=100&query="+url.QueryEscape(email), &body); err != nil {
		return "", err
	}
	for _, u := range body.Users {
		if strings.EqualFold(u.Email, email) {
			return u.Name, nil
		}
	}
	return "", fmt.Errorf("%w: %s", errNoUser, email)
}

// normRegion maps user input to "us" or "eu"; "" if unrecognised.
func normRegion(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "us", "u", "usa":
		return "us"
	case "eu", "e":
		return "eu"
	}
	return ""
}

// applyRegion sets the env Terraform's provider reads: "eu", or unset for US.
func applyRegion(region string) {
	if region == "eu" {
		os.Setenv(envRegion, "eu")
	} else {
		os.Unsetenv(envRegion)
	}
}

// tfvarsEmail reads responder_email from terraform/terraform.tfvars, if any.
func tfvarsEmail(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "terraform", "terraform.tfvars"))
	if err != nil {
		return ""
	}
	m := tfvarsRe.FindSubmatch(raw)
	if m == nil || string(m[1]) == "you@example.com" {
		return ""
	}
	return string(m[1])
}

func mask(tok string) string {
	if len(tok) <= 4 {
		return "****"
	}
	return "****" + tok[len(tok)-4:]
}

func ask(prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func askYesNo(prompt string, def bool) (bool, error) {
	hint := " [Y/n]: "
	if !def {
		hint = " [y/N]: "
	}
	for {
		ans, err := ask(prompt + hint)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(ans) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		fmt.Println("  Type y or n.")
	}
}
