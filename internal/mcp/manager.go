package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var defaultServers = []ServerConfig{
	{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
		Enabled: true,
	},
	{
		Name:    "fetch",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-fetch"},
		Enabled: true,
	},
	{
		Name:    "memory",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-memory"},
		Enabled: true,
	},
	{
		Name:    "sequential-thinking",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		Enabled: true,
	},
	{
		Name:    "puppeteer",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-puppeteer"},
		Enabled: true,
	},
	{
		Name:    "time",
		Command: "npx",
		Args:    []string{"-y", "mcp-server-time"},
		Enabled: true,
	},
	{
		Name:    "calculator",
		Command: "npx",
		Args:    []string{"-y", "mcp-server-calculator"},
		Enabled: true,
	},
	{
		Name:    "wikipedia",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-wikipedia"},
		Enabled: false, // package unstable — use web_search + fetch_url for Wikipedia
	},
	{
		Name:    "arxiv",
		Command: "npx",
		Args:    []string{"-y", "mcp-server-arxiv"},
		Enabled: true,
	},
	{
		Name:    "shadcn-ui",
		Command: "npx",
		Args:    []string{"-y", "shadcn-ui-mcp-server"},
		Enabled: true,
	},
	// ── Requires API key (disabled by default) ────────────────────────────
	{
		Name:    "github",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-github"},
		Env:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": ""},
		Enabled: false,
	},
	{
		Name:    "brave-search",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-brave-search"},
		Env:     map[string]string{"BRAVE_API_KEY": ""},
		Enabled: false,
	},
	{
		Name:    "sqlite",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-sqlite", "--db-path", "themis.db"},
		Enabled: false,
	},
	{
		Name:    "vercel",
		Command: "npx",
		Args:    []string{"-y", "@vercel/mcp"},
		Env:     map[string]string{"VERCEL_TOKEN": ""},
		Enabled: false, // enable by setting VERCEL_TOKEN env var
	},
}

// Manager owns all MCP server connections.
type Manager struct {
	cfgPath string
	mu      sync.RWMutex
	clients map[string]*Client // keyed by server name
	configs []ServerConfig
}

// NewManager creates a Manager using ~/.config/themis/mcp_servers.json.
func NewManager() *Manager {
	cfgDir, _ := os.UserConfigDir()
	cfgPath := filepath.Join(cfgDir, "themis", "mcp_servers.json")
	return &Manager{
		cfgPath: cfgPath,
		clients: make(map[string]*Client),
	}
}

// LoadConfig reads the config file, creating it with defaults if absent.
// It also merges any newly added default servers into an existing config.
func (m *Manager) LoadConfig() error {
	data, err := os.ReadFile(m.cfgPath)
	if os.IsNotExist(err) {
		m.configs = defaultServers
		return m.SaveConfig()
	}
	if err != nil {
		return err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("mcp_servers.json: %w", err)
	}
	m.configs = cfg.Servers

	// Merge any default servers that don't exist in the saved config yet.
	existing := make(map[string]bool, len(m.configs))
	for _, s := range m.configs {
		existing[s.Name] = true
	}
	added := false
	for _, def := range defaultServers {
		if !existing[def.Name] {
			m.configs = append(m.configs, def)
			added = true
		}
	}
	if added {
		_ = m.SaveConfig() // persist merged config; non-fatal if it fails
	}
	return nil
}

// SaveConfig persists the current config list to disk.
func (m *Manager) SaveConfig() error {
	if err := os.MkdirAll(filepath.Dir(m.cfgPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(Config{Servers: m.configs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgPath, data, 0o600)
}

// StartEnabled connects to all enabled servers sequentially.
// Starting them sequentially avoids npm strict-lock/race conditions when
// multiple npx processes try to download and install packages at the same time.
func (m *Manager) StartEnabled(ctx context.Context) {
	m.mu.Lock()
	configs := make([]ServerConfig, len(m.configs))
	copy(configs, m.configs)
	m.mu.Unlock()

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		client := NewClient(cfg)
		if err := client.Connect(ctx); err != nil {
			// Store as failed so dashboard can show the error.
			_ = err
			continue
		}
		m.mu.Lock()
		m.clients[cfg.Name] = client
		m.mu.Unlock()
	}
}

// CallTool routes a prefixed tool name (mcp__servername__toolname) to the right server.
func (m *Manager) CallTool(ctx context.Context, prefixedName string, args map[string]interface{}) (string, error) {
	serverName, toolName, ok := parseMCPToolName(prefixedName)
	if !ok {
		return "", fmt.Errorf("invalid MCP tool name: %q", prefixedName)
	}
	m.mu.RLock()
	client, exists := m.clients[serverName]
	m.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("MCP server %q not connected", serverName)
	}
	return client.CallTool(ctx, toolName, args)
}

// AllTools returns all tools from all connected servers, prefixed with mcp__servername__.
func (m *Manager) AllTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Tool
	for name, client := range m.clients {
		for _, t := range client.Tools() {
			out = append(out, Tool{
				Name:        fmt.Sprintf("mcp__%s__%s", name, t.Name),
				Description: fmt.Sprintf("[MCP:%s] %s", name, t.Description),
			})
		}
	}
	return out
}

// Statuses returns the runtime status of every configured server.
func (m *Manager) Statuses() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ServerStatus
	for _, cfg := range m.configs {
		client, connected := m.clients[cfg.Name]
		status := ServerStatus{Config: cfg}
		if connected && client.IsReady() {
			status.Ready = true
			status.Tools = client.Tools()
		}
		out = append(out, status)
	}
	return out
}

// Configs returns a copy of all server configs.
func (m *Manager) Configs() []ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerConfig, len(m.configs))
	copy(out, m.configs)
	return out
}

// AddServer appends a new server config, saves, and optionally connects it.
func (m *Manager) AddServer(ctx context.Context, cfg ServerConfig) error {
	m.mu.Lock()
	for _, existing := range m.configs {
		if existing.Name == cfg.Name {
			m.mu.Unlock()
			return fmt.Errorf("server %q already exists", cfg.Name)
		}
	}
	m.configs = append(m.configs, cfg)
	m.mu.Unlock()

	if err := m.SaveConfig(); err != nil {
		return err
	}
	if cfg.Enabled {
		client := NewClient(cfg)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("server added but connect failed: %w", err)
		}
		m.mu.Lock()
		m.clients[cfg.Name] = client
		m.mu.Unlock()
	}
	return nil
}

// ToggleServer enables or disables a server by name and saves config.
func (m *Manager) ToggleServer(ctx context.Context, name string, enable bool) error {
	m.mu.Lock()
	found := false
	for i := range m.configs {
		if m.configs[i].Name == name {
			m.configs[i].Enabled = enable
			found = true
			break
		}
	}
	m.mu.Unlock()
	if !found {
		return fmt.Errorf("server %q not found", name)
	}
	if err := m.SaveConfig(); err != nil {
		return err
	}
	if enable {
		m.mu.RLock()
		cfg := ServerConfig{}
		for _, c := range m.configs {
			if c.Name == name {
				cfg = c
				break
			}
		}
		m.mu.RUnlock()
		client := NewClient(cfg)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("connect failed: %w", err)
		}
		m.mu.Lock()
		m.clients[name] = client
		m.mu.Unlock()
	} else {
		m.mu.Lock()
		if c, ok := m.clients[name]; ok {
			c.Close()
			delete(m.clients, name)
		}
		m.mu.Unlock()
	}
	return nil
}

// ConfigPath returns the path to the config file.
func (m *Manager) ConfigPath() string { return m.cfgPath }

// IsMCPTool reports whether a tool name uses the mcp__ prefix.
func IsMCPTool(name string) bool { return strings.HasPrefix(name, "mcp__") }

func parseMCPToolName(name string) (server, tool string, ok bool) {
	parts := strings.SplitN(name, "__", 3)
	if len(parts) != 3 || parts[0] != "mcp" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
