// Package mcp provides a Model Context Protocol client for connecting to MCP servers.
package mcp

import "encoding/json"

const protocolVersion = "2024-11-05"

// ── JSON-RPC 2.0 wire types ───────────────────────────────────────────────

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── MCP protocol params/results ───────────────────────────────────────────

type initializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    struct{}   `json:"capabilities"`
	ClientInfo      clientInfo `json:"clientInfo"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError"`
}

// ── Public types ──────────────────────────────────────────────────────────

// Tool is an MCP tool exposed by a server.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ContentItem is one piece of content in a tool response.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ServerConfig defines how to launch an MCP server process.
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
}

// Config is the top-level structure stored in mcp_servers.json.
type Config struct {
	Servers []ServerConfig `json:"servers"`
}

// ServerStatus is the runtime status of one server.
type ServerStatus struct {
	Config ServerConfig
	Ready  bool
	Err    error
	Tools  []Tool
}
