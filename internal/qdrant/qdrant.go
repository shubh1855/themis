package qdrant

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	qdrantVersion = "v1.17.1"
	qdrantPort    = "6333"
	healthURL     = "http://127.0.0.1:" + qdrantPort + "/healthz"
	baseAPIURL    = "http://127.0.0.1:" + qdrantPort
)

// Status represents the current state of the Qdrant daemon.
type Status int

const (
	StatusStopped Status = iota
	StatusDownloading
	StatusStarting
	StatusReady
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusDownloading:
		return "downloading"
	case StatusStarting:
		return "starting"
	case StatusReady:
		return "ready"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Manager handles auto-downloading and lifecycle management of the Qdrant binary.
type Manager struct {
	mu      sync.RWMutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	binPath string
	dataDir string
	status  Status
	lastErr error
}

func New() *Manager {
	base := platformDataDir()

	exe := "qdrant"
	if runtime.GOOS == "windows" {
		exe = "qdrant.exe"
	}

	return &Manager{
		binPath: filepath.Join(base, "bin", exe),
		dataDir: filepath.Join(base, "qdrant_storage"),
		status:  StatusStopped,
	}
}

func platformDataDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "themis")
		}
	} else if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "themis")
		}
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "themis")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "themis")
	}
	return filepath.Join(".", "themis")
}

// Status returns the current daemon status.
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// LastError returns the last error encountered.
func (m *Manager) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

// BaseURL returns the HTTP API base URL.
func (m *Manager) BaseURL() string {
	return baseAPIURL
}

func (m *Manager) setStatus(s Status, err error) {
	m.mu.Lock()
	m.status = s
	m.lastErr = err
	m.mu.Unlock()
}

// EnsureRunning downloads Qdrant if missing and starts the daemon.
// This is safe to call from a goroutine. It blocks until Qdrant is healthy or fails.
func (m *Manager) EnsureRunning() error {
	_ = os.MkdirAll(filepath.Dir(m.binPath), 0755)
	_ = os.MkdirAll(m.dataDir, 0755)

	// Check if something is already listening on the port
	if m.isHealthy() {
		m.setStatus(StatusReady, nil)
		return nil
	}

	// Check if binary exists and is a real executable (not a shell script mock)
	needsDownload := false
	info, err := os.Stat(m.binPath)
	if os.IsNotExist(err) {
		needsDownload = true
	} else if err == nil && info.Size() < 1_000_000 {
		// The mock placeholder is tiny; real qdrant binary is ~30MB
		needsDownload = true
	}

	if needsDownload {
		m.setStatus(StatusDownloading, nil)
		if err := m.download(); err != nil {
			m.setStatus(StatusFailed, err)
			return fmt.Errorf("qdrant download: %w", err)
		}
	}

	// Write a minimal config so Qdrant uses our data directory
	configPath := filepath.Join(filepath.Dir(m.binPath), "config.yaml")
	configContent := fmt.Sprintf(`storage:
  storage_path: %s
  snapshots_path: %s/snapshots
service:
  host: 127.0.0.1
  http_port: 6333
  grpc_port: 6334
telemetry_disabled: true
`, m.dataDir, m.dataDir)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	// Start the daemon
	m.setStatus(StatusStarting, nil)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.cmd = exec.CommandContext(ctx, m.binPath, "--config-path", configPath, "--disable-telemetry")
	m.cmd.Env = append(os.Environ(),
		"QDRANT__STORAGE__STORAGE_PATH="+m.dataDir,
		"QDRANT__SERVICE__HOST=127.0.0.1",
		"QDRANT__SERVICE__HTTP_PORT=6333",
	)
	// Log stderr to a file for debugging
	logPath := filepath.Join(filepath.Dir(m.binPath), "qdrant.log")
	logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	m.cmd.Stdout = logFile
	m.cmd.Stderr = logFile
	m.cmd.Stdin = nil

	if err := m.cmd.Start(); err != nil {
		m.setStatus(StatusFailed, err)
		cancel()
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("qdrant start: %w", err)
	}

	// Monitor the process in the background so we detect crashes
	go func() {
		_ = m.cmd.Wait()
		m.setStatus(StatusStopped, nil)
	}()

	// Poll for health with timeout
	if err := m.waitForHealthy(20 * time.Second); err != nil {
		m.setStatus(StatusFailed, err)
		return err
	}

	m.setStatus(StatusReady, nil)
	return nil
}

// Stop kills the Qdrant daemon.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
	}
	m.setStatus(StatusStopped, nil)
}

func (m *Manager) isHealthy() bool {
	// Quick TCP check first
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+qdrantPort, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()

	// Then HTTP health check
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func (m *Manager) waitForHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.isHealthy() {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("qdrant failed to become healthy within %s", timeout)
}

func (m *Manager) download() error {
	url := releaseURL()
	if url == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("⬇  Downloading Qdrant %s for %s/%s...\n", qdrantVersion, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("   URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	_ = os.Remove(m.binPath)

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "qdrant-*")
	if err != nil {
		return fmt.Errorf("create temp download file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("download body copy: %w", err)
	}
	tmpFile.Close()

	if strings.HasSuffix(url, ".zip") {
		zr, err := zip.OpenReader(tmpName)
		if err != nil {
			return fmt.Errorf("zip open: %w", err)
		}
		defer zr.Close()
		for _, file := range zr.File {
			base := filepath.Base(file.Name)
			if base == "qdrant.exe" {
				rc, err := file.Open()
				if err != nil {
					return fmt.Errorf("zip file open: %w", err)
				}
				f, err := os.OpenFile(m.binPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					rc.Close()
					return fmt.Errorf("create binary: %w", err)
				}
				n, err := io.Copy(f, rc)
				f.Close()
				rc.Close()
				if err != nil {
					return fmt.Errorf("write binary: %w", err)
				}
				fmt.Printf("✓  Qdrant installed to %s (%d MB)\n", m.binPath, n/1_000_000)
				return nil
			}
		}
	} else {
		f, err := os.Open(tmpName)
		if err != nil {
			return fmt.Errorf("reopen temp file: %w", err)
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()

		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("tar read: %w", err)
			}

			base := filepath.Base(hdr.Name)
			if (base == "qdrant" || base == "qdrant.exe") && hdr.Typeflag == tar.TypeReg {
				out, err := os.OpenFile(m.binPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return fmt.Errorf("create binary: %w", err)
				}
				n, err := io.Copy(out, tr)
				out.Close()
				if err != nil {
					return fmt.Errorf("write binary: %w", err)
				}
				fmt.Printf("✓  Qdrant installed to %s (%d MB)\n", m.binPath, n/1_000_000)
				return nil
			}
		}
	}

	return fmt.Errorf("qdrant binary not found in the downloaded archive")
}

func releaseURL() string {
	base := fmt.Sprintf("https://github.com/qdrant/qdrant/releases/download/%s", qdrantVersion)
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return base + "/qdrant-x86_64-unknown-linux-musl.tar.gz"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return base + "/qdrant-aarch64-unknown-linux-musl.tar.gz"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return base + "/qdrant-x86_64-apple-darwin.tar.gz"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return base + "/qdrant-aarch64-apple-darwin.tar.gz"
	case runtime.GOOS == "windows" && strings.Contains(runtime.GOARCH, "amd64"):
		return base + "/qdrant-x86_64-pc-windows-msvc.zip"
	default:
		return ""
	}
}
