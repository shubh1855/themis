package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type SSRFChecker struct {
	AllowPrivate bool
	AllowedHosts map[string]bool
	BlockedHosts map[string]bool
}

func NewSSRFChecker() *SSRFChecker {
	return &SSRFChecker{
		AllowedHosts: make(map[string]bool),
		BlockedHosts: make(map[string]bool),
	}
}

func (c *SSRFChecker) Check(rawURL string) error {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return err
	}

	host := u.Hostname()

	if c.BlockedHosts[strings.ToLower(host)] {
		return fmt.Errorf("ssrf: host %q is blocked", host)
	}

	if len(c.AllowedHosts) > 0 && !c.AllowedHosts[strings.ToLower(host)] {
		return fmt.Errorf("ssrf: host %q not in allowed list", host)
	}

	if c.AllowPrivate {
		return nil
	}

	if isPrivateHostname(host) {
		return fmt.Errorf("ssrf: private hostname %q", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("ssrf: private IP %q", host)
		}
	}

	return nil
}

func (c *SSRFChecker) CheckResolved(rawURL string) error {
	if err := c.Check(rawURL); err != nil {
		return err
	}

	u, _ := url.Parse(rawURL)
	host := u.Hostname()

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
