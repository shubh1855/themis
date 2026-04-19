package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a single MCP server connection over stdio.
type Client struct {
	cfg     ServerConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	pending map[int]chan jsonrpcResponse
	nextID  atomic.Int32
	tools   []Tool
	ready   bool
	done    chan struct{}
}

// NewClient creates a client for the given config. Call Connect to start it.
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		cfg:     cfg,
		pending: make(map[int]chan jsonrpcResponse),
		done:    make(chan struct{}),
	}
}

// Connect starts the server subprocess and performs the MCP handshake.
func (c *Client) Connect(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.cfg.Command, c.cfg.Args...)

	// Merge parent env with server-specific overrides.
	env := os.Environ()
	for k, v := range c.cfg.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdout pipe: %w", err)
	}
	// Discard stderr to avoid noise in the TUI.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp start %q: %w", c.cfg.Command, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	go c.readLoop(bufio.NewScanner(stdout))

	if err := c.initialize(ctx); err != nil {
		c.Close()
		return fmt.Errorf("mcp handshake %q: %w", c.cfg.Name, err)
	}
	c.ready = true
	return nil
}

func (c *Client) readLoop(scanner *bufio.Scanner) {
	defer func() {
		// Drain any waiting callers.
		c.mu.Lock()
		for _, ch := range c.pending {
			close(ch)
		}
		c.pending = make(map[int]chan jsonrpcResponse)
		c.mu.Unlock()
		close(c.done)
	}()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (c *Client) request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan jsonrpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	c.mu.Lock()
	_, writeErr := fmt.Fprintf(c.stdin, "%s\n", data)
	c.mu.Unlock()
	if writeErr != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write: %w", writeErr)
	}

	timeout := 30 * time.Second
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("server closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("request timeout (%s)", timeout)
	}
}

func (c *Client) notify(method string, params interface{}) {
	req := jsonrpcRequest{JSONRPC: "2.0", Method: method, Params: params}
	data, _ := json.Marshal(req)
	c.mu.Lock()
	fmt.Fprintf(c.stdin, "%s\n", data) //nolint:errcheck
	c.mu.Unlock()
}

func (c *Client) initialize(ctx context.Context) error {
	params := initializeParams{
		ProtocolVersion: protocolVersion,
		ClientInfo:      clientInfo{Name: "themis", Version: "1.0.0"},
	}
	raw, err := c.request(ctx, "initialize", params)
	if err != nil {
		return err
	}
	_ = raw // server capabilities not needed

	c.notify("notifications/initialized", struct{}{})

	return c.listTools(ctx)
}

func (c *Client) listTools(ctx context.Context) error {
	raw, err := c.request(ctx, "tools/list", struct{}{})
	if err != nil {
		return err
	}
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse tools/list: %w", err)
	}
	c.tools = result.Tools
	return nil
}

// CallTool invokes a tool on this server and returns concatenated text content.
func (c *Client) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if !c.ready {
		return "", fmt.Errorf("server %q not ready", c.cfg.Name)
	}
	params := toolCallParams{Name: toolName, Arguments: args}
	raw, err := c.request(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}
	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse tools/call result: %w", err)
	}
	if result.IsError {
		var parts []string
		for _, item := range result.Content {
			if item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		return "", fmt.Errorf("tool error: %s", strings.Join(parts, "; "))
	}
	var sb strings.Builder
	for _, item := range result.Content {
		if item.Type == "text" && item.Text != "" {
			sb.WriteString(item.Text)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// Tools returns the tools advertised by this server.
func (c *Client) Tools() []Tool { return c.tools }

// IsReady reports whether the server is connected and initialized.
func (c *Client) IsReady() bool { return c.ready }

// Name returns the server's configured name.
func (c *Client) Name() string { return c.cfg.Name }

// Close terminates the server process.
func (c *Client) Close() {
	c.ready = false
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill() //nolint:errcheck
	}
}
