package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// SSRFChecker validates URLs against SSRF attacks before making outbound requests.
type SSRFChecker struct {
	// AllowPrivate disables private IP blocking (for testing only).
	AllowPrivate bool
	// AllowedHosts is an optional allow-list; if non-empty only these hosts pass.
	AllowedHosts map[string]bool
	// BlockedHosts is an explicit deny-list.
	BlockedHosts map[string]bool
}

// NewSSRFChecker creates a default SSRF checker that blocks all private ranges.
func NewSSRFChecker() *SSRFChecker {
	return &SSRFChecker{
		AllowedHosts: make(map[string]bool),
		BlockedHosts: make(map[string]bool),
	}
}

// Check validates a URL is safe to fetch. Returns nil if safe.
func (c *SSRFChecker) Check(rawURL string) error {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return err
	}

	host := u.Hostname()

	// Explicit block list
	if c.BlockedHosts[strings.ToLower(host)] {
		return fmt.Errorf("ssrf: host %q is blocked", host)
	}

	// If allow-list is set, enforce it
	if len(c.AllowedHosts) > 0 && !c.AllowedHosts[strings.ToLower(host)] {
		return fmt.Errorf("ssrf: host %q not in allowed list", host)
	}

	if c.AllowPrivate {
		return nil
	}

	// Check hostname
	if isPrivateHostname(host) {
		return fmt.Errorf("ssrf: private hostname %q", host)
	}

	// Check for IP literals
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("ssrf: private IP %q", host)
		}
	}

	return nil
}

// CheckResolved performs the full SSRF check including DNS resolution.
func (c *SSRFChecker) CheckResolved(rawURL string) error {
	if err := c.Check(rawURL); err != nil {
		return err
	}

	u, _ := url.Parse(rawURL)
	host := u.Hostname()

	// Skip DNS for IP literals
	if net.ParseIP(host) != nil {
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("ssrf: cannot resolve %q: %w", host, err)
	}

	if !c.AllowPrivate {
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return fmt.Errorf("ssrf: host %q resolves to private IP %s", host, ip)
			}
		}
	}

	return nil
}
