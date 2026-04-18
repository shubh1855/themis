package security_test

import (
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/security"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com", false},
		{"file scheme blocked", "file:///etc/passwd", true},
		{"ftp blocked", "ftp://example.com", true},
		{"empty url", "", true},
		{"no scheme", "example.com", true},
		{"no host", "https://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := security.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestSSRFChecker(t *testing.T) {
	checker := security.NewSSRFChecker()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"public url", "https://example.com", false},
		{"localhost", "http://localhost:8080", true},
		{"127.0.0.1", "http://127.0.0.1:3000", true},
		{"ipv6 loopback", "http://[::1]:8080", true},
		{"private 10.x", "http://10.0.0.1", true},
		{"private 192.168.x", "http://192.168.1.1", true},
		{"private 172.16.x", "http://172.16.0.1", true},
		{"file scheme", "file:///etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker.Check(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		path    string
		wantErr bool
	}{
		{"safe relative", "/home/user/project", "src/main.go", false},
		{"traversal attack", "/home/user/project", "../../../etc/passwd", true},
		{"dot-dot in middle", "/home/user/project", "src/../../etc/passwd", true},
		{"absolute within root", "/home/user/project", "/home/user/project/src/main.go", false},
		{"absolute outside root", "/home/user/project", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := security.SanitizePath(tt.root, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePath(%q, %q) error = %v, wantErr %v", tt.root, tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "passwd"},
		{".hidden", "hidden"},
		{"", "unnamed"},
		{"..", "unnamed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := security.SafeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("SafeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
