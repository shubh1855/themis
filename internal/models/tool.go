package models

import "fmt"

type ToolRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func SuccessResponse(data interface{}) ToolResponse {
	return ToolResponse{Success: true, Data: data}
}

func ErrorResponse(msg string) ToolResponse {
	return ToolResponse{Success: false, Error: msg}
}

func ErrorResponsef(format string, args ...interface{}) ToolResponse {
	return ToolResponse{Success: false, Error: fmt.Sprintf(format, args...)}
}
