package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

var AllowedSchemes = map[string]bool{
	"http":  true,
	"https": true,
}

func ValidateURL(rawURL string) (*url.URL, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("security: empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("security: invalid URL %q: %w", rawURL, err)
	}

	if u.Scheme == "" {
		return nil, fmt.Errorf("security: URL missing scheme: %q", rawURL)
	}

	if !AllowedSchemes[strings.ToLower(u.Scheme)] {
		return nil, fmt.Errorf("security: blocked scheme %q in URL %q", u.Scheme, rawURL)
	}

	if u.Host == "" {
		return nil, fmt.Errorf("security: URL missing host: %q", rawURL)
	}

	return u, nil
}

func IsBlockedHost(rawURL string) (bool, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return true, err
	}

	host := u.Hostname()

	if isPrivateHostname(host) {
		return true, fmt.Errorf("security: blocked private host %q", host)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return true, fmt.Errorf("security: cannot resolve host %q: %w", host, err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true, fmt.Errorf("security: host %q resolves to private IP %s", host, ip)
		}
	}

	return false, nil
}

func isPrivateHostname(host string) bool {
	lower := strings.ToLower(host)
	privates := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"[::1]",
	}
	for _, p := range privates {
		if lower == p {
			return true
		}
	}
	for _, suffix := range []string{".local", ".internal", ".localhost"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{mustParseCIDR("10.0.0.0/8")},
		{mustParseCIDR("172.16.0.0/12")},
		{mustParseCIDR("192.168.0.0/16")},
		{mustParseCIDR("127.0.0.0/8")},
		{mustParseCIDR("169.254.0.0/16")},
		{mustParseCIDR("fc00::/7")},
		{mustParseCIDR("fe80::/10")},
		{mustParseCIDR("::1/128")},
	}
	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("security: bad CIDR: " + s)
	}
	return n
}
