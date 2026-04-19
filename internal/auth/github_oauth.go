// Package auth implements GitHub authentication for the Themis CLI.
//
// Authentication strategy (in priority order):
//  1. Stored Themis token: ~/.config/themis/github_token.json
//  2. gh CLI auth: uses `gh auth token` if gh is installed and authenticated
//  3. Env vars: GH_TOKEN or GITHUB_TOKEN
//
// For login, we prefer `gh auth login` (which handles OAuth Device Flow
// correctly with a pre-registered app) over our own Device Flow.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StoredToken is the token data persisted to disk.
type StoredToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
	Username    string    `json:"username"`
}

// ── Token resolution ─────────────────────────────────────────────────────────

// GetToken returns the best available GitHub token via priority chain.
func GetToken() string {
	// 1. Themis stored token
	if tok, err := LoadToken(); err == nil && tok.AccessToken != "" {
		return tok.AccessToken
	}
	// 2. gh CLI
	if tok := ghCliToken(); tok != "" {
		return tok
	}
	// 3. Environment variables
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		return tok
	}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return tok
	}
	return ""
}

// IsLoggedIn returns true if any authentication source has a valid token.
func IsLoggedIn() bool {
	return GetToken() != ""
}

// GetUsername returns the authenticated GitHub username, or "" if not found.
func GetUsername() string {
	// Check stored Themis token first
	if tok, err := LoadToken(); err == nil && tok.Username != "" {
		return tok.Username
	}
	// Try to fetch from GitHub API using any available token
	token := GetToken()
	if token == "" {
		return ""
	}
	return fetchGitHubUsername(token)
}

// ── gh CLI bridge ─────────────────────────────────────────────────────────────

// ghCliToken runs `gh auth token` and returns the token if gh is authenticated.
func ghCliToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ── Login flow ────────────────────────────────────────────────────────────────

// Login authenticates the user with GitHub.
// It first checks if gh CLI is already authenticated. If not, it uses Device Flow.
func Login(ctx context.Context) (instructions string, token string, err error) {
	// Try to get token from existing gh cli session
	if tok := ghCliToken(); tok != "" {
		username := fetchGitHubUsername(tok)
		stored := StoredToken{
			AccessToken: tok,
			TokenType:   "bearer",
			Scope:       "repo",
			CreatedAt:   time.Now().UTC(),
			Username:    username,
		}
		_ = SaveToken(&stored)
		os.Setenv("GH_TOKEN", tok)
		os.Setenv("GITHUB_TOKEN", tok)
		return "Authenticated securely via existing gh CLI configuration.", tok, nil
	}

	// Otherwise, fall back to Device Flow which is non-interactive
	return loginDeviceFlow(ctx)
}

// Logout removes the stored Themis token. Does not log out of gh CLI.
func Logout() error {
	path, err := tokenPath()
	if err != nil {
		return err
	}
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ── Device Flow fallback ──────────────────────────────────────────────────────

const (
	deviceCodeURL   = "https://github.com/login/device/code"
	tokenURL        = "https://github.com/login/oauth/access_token"
)

// deviceClientID returns the OAuth App Client ID from env or gh's own.
func deviceClientID() string {
	if id := os.Getenv("THEMIS_GITHUB_CLIENT_ID"); id != "" {
		return id
	}
	// gh CLI's public client ID (works without registering an app)
	return "178c6fc778ccc68e1d6a"
}

// DeviceCodeResponse is returned by GitHub's device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenPollResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

func loginDeviceFlow(ctx context.Context) (instructions string, token string, err error) {
	dc, err := requestDeviceCode(ctx)
	if err != nil {
		return "", "", fmt.Errorf(
			"GitHub OAuth Device Flow failed: %w\n\nAlternatively, install gh CLI and run: gh auth login", err)
	}

	instructions = fmt.Sprintf(
		"Open %s in your browser and enter this code: %s",
		dc.VerificationURI, dc.UserCode,
	)

	tok, err := pollForToken(ctx, dc.DeviceCode, dc.Interval)
	if err != nil {
		return instructions, "", err
	}

	username := fetchGitHubUsername(tok.AccessToken)
	stored := StoredToken{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		Scope:       tok.Scope,
		CreatedAt:   time.Now().UTC(),
		Username:    username,
	}
	if err := SaveToken(&stored); err != nil {
		return instructions, "", fmt.Errorf("auth: save token: %w", err)
	}

	os.Setenv("GH_TOKEN", tok.AccessToken)
	os.Setenv("GITHUB_TOKEN", tok.AccessToken)

	return instructions, tok.AccessToken, nil
}

func requestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	body := strings.NewReader(fmt.Sprintf("client_id=%s&scope=repo", deviceClientID()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var dc DeviceCodeResponse
	if err := json.Unmarshal(data, &dc); err != nil {
		return nil, fmt.Errorf("parse device code: %w", err)
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("no device_code returned — check THEMIS_GITHUB_CLIENT_ID or install gh CLI")
	}
	return &dc, nil
}

func pollForToken(ctx context.Context, deviceCode string, interval int) (*tokenPollResponse, error) {
	if interval <= 0 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			tok, err := pollOnce(ctx, deviceCode)
			if err != nil {
				return nil, err
			}
			switch tok.Error {
			case "":
				return tok, nil
			case "authorization_pending":
				// continuing to poll
			case "slow_down":
				interval += 5
				ticker.Reset(time.Duration(interval) * time.Second)
			case "expired_token":
				return nil, fmt.Errorf("device code expired — run github_login again")
			case "access_denied":
				return nil, fmt.Errorf("access denied by user")
			default:
				return nil, fmt.Errorf("unexpected error: %s", tok.Error)
			}
		}
	}
}

func pollOnce(ctx context.Context, deviceCode string) (*tokenPollResponse, error) {
	body := strings.NewReader(fmt.Sprintf(
		"client_id=%s&device_code=%s&grant_type=urn:ietf:params:oauth:grant-type:device_code",
		deviceClientID(), deviceCode,
	))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tok tokenPollResponse
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &tok, nil
}

// ── Token persistence ─────────────────────────────────────────────────────────

func tokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "themis", "github_token.json"), nil
}

// SaveToken writes the token to disk with 0600 permissions.
func SaveToken(token *StoredToken) error {
	path, err := tokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads the stored token from disk.
func LoadToken() (*StoredToken, error) {
	path, err := tokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok StoredToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func fetchGitHubUsername(token string) string {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var user struct {
		Login string `json:"login"`
	}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &user)
	return user.Login
}
