// Package models defines shared data structures used across the agent tool system.
package models

import "fmt"

// ToolRequest represents an incoming tool invocation from the agent.
type ToolRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

// ToolResponse is the standard response envelope for all tool executions.
type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SuccessResponse creates a successful ToolResponse with the given data.
func SuccessResponse(data interface{}) ToolResponse {
	return ToolResponse{Success: true, Data: data}
}

// ErrorResponse creates a failed ToolResponse with the given error message.
func ErrorResponse(msg string) ToolResponse {
	return ToolResponse{Success: false, Error: msg}
}

// ErrorResponsef creates a failed ToolResponse with a formatted error message.
func ErrorResponsef(format string, args ...interface{}) ToolResponse {
	return ToolResponse{Success: false, Error: fmt.Sprintf(format, args...)}
}
