// Package auth implements GitHub OAuth Device Flow for CLI authentication.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	deviceCodeURL   = "https://github.com/login/device/code"
	tokenURL        = "https://github.com/login/oauth/access_token"
	defaultClientID = "Ov23liGg5X2AoCl9DreF"
)

// DeviceCodeResponse is returned by the GitHub device code endpoint.
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

// StoredToken is the token data persisted to disk.
type StoredToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
	Username    string    `json:"username"`
}

func clientID() string {
	if id := os.Getenv("THEMIS_GITHUB_CLIENT_ID"); id != "" {
		return id
	}
	return defaultClientID
}

func tokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "themis", "github_token.json"), nil
}

// RequestDeviceCode requests a device code from GitHub.
func RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	vals := url.Values{
		"client_id": {clientID()},
		"scope":     {"repo"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(vals.Encode()))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var dc DeviceCodeResponse
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("auth: parse device code response: %w", err)
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("auth: no device code returned — set THEMIS_GITHUB_CLIENT_ID")
	}
	return &dc, nil
}

// PollForToken polls GitHub until the user completes authorization or an error occurs.
func PollForToken(ctx context.Context, deviceCode string, interval int) (*tokenPollResponse, error) {
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
				// user has not yet authorized
			case "slow_down":
				interval += 5
				ticker.Reset(time.Duration(interval) * time.Second)
			case "expired_token":
				return nil, fmt.Errorf("auth: device code expired — run github_login again")
			case "access_denied":
				return nil, fmt.Errorf("auth: access denied by user")
			default:
				return nil, fmt.Errorf("auth: unexpected error: %s", tok.Error)
			}
		}
	}
}

func pollOnce(ctx context.Context, deviceCode string) (*tokenPollResponse, error) {
	vals := url.Values{
		"client_id":   {clientID()},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(vals.Encode()))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tok tokenPollResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("auth: parse token response: %w", err)
	}
	return &tok, nil
}

// Login runs the full GitHub Device Flow. Blocks until the user authorizes.
// Returns the display instructions and the access token once authentication completes.
func Login(ctx context.Context) (instructions string, token string, err error) {
	dc, err := RequestDeviceCode(ctx)
	if err != nil {
		return "", "", fmt.Errorf("auth: request device code: %w", err)
	}

	instructions = fmt.Sprintf("Open %s and enter code: %s", dc.VerificationURI, dc.UserCode)

	tok, err := PollForToken(ctx, dc.DeviceCode, dc.Interval)
	if err != nil {
		return instructions, "", err
	}

	stored := StoredToken{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		Scope:       tok.Scope,
		CreatedAt:   time.Now().UTC(),
	}
	if err := SaveToken(&stored); err != nil {
		return instructions, "", fmt.Errorf("auth: save token: %w", err)
	}

	os.Setenv("GH_TOKEN", tok.AccessToken)
	os.Setenv("GITHUB_TOKEN", tok.AccessToken)

	return instructions, tok.AccessToken, nil
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

// IsLoggedIn returns true if a valid stored token exists.
func IsLoggedIn() bool {
	tok, err := LoadToken()
	return err == nil && tok.AccessToken != ""
}

// GetToken returns the stored access token string.
func GetToken() (string, error) {
	tok, err := LoadToken()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// Logout deletes the stored token file.
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
