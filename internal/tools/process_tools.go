package tools

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

var processTracker = system.NewProcessTracker()

// HandleRunCmd executes a command and returns its output.
func HandleRunCmd(ctx Context) models.ToolResponse {
	command := models.ArgString(ctx.Req.Args, "command")
	if command == "" {
		return models.ErrorResponse("run_cmd: missing 'command' argument")
	}

	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	timeoutSec := models.ArgInt(ctx.Req.Args, "timeout", 60)
	timeout := time.Duration(timeoutSec) * time.Second

	result, err := system.RunShellCmd(context.Background(), command, dir, timeout)
	if err != nil {
		return models.ErrorResponsef("run_cmd: %v", err)
	}
	return models.SuccessResponse(result)
}

// HandleRunFile runs a file using the appropriate interpreter.
func HandleRunFile(ctx Context) models.ToolResponse {
	path := models.ArgString(ctx.Req.Args, "path")
	if path == "" {
		return models.ErrorResponse("run_file: missing 'path' argument")
	}

	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	// Build command based on file extension
	command := buildRunCommand(path)
	if command == "" {
		return models.ErrorResponse("run_file: unsupported file type")
	}

	result, err := system.RunShellCmd(context.Background(), command, dir, system.DefaultCmdTimeout)
	if err != nil {
		return models.ErrorResponsef("run_file: %v", err)
	}
	return models.SuccessResponse(result)
}

// HandleStartBackground starts a command in the background.
func HandleStartBackground(ctx Context) models.ToolResponse {
	command := models.ArgString(ctx.Req.Args, "command")
	if command == "" {
		return models.ErrorResponse("start_background: missing 'command' argument")
	}

	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	_, shellBin := system.DetectShell()
	result, err := processTracker.StartBackground(context.Background(), shellBin, []string{"-c", command}, dir)
	if err != nil {
		return models.ErrorResponsef("start_background: %v", err)
	}
	return models.SuccessResponse(result)
}

// HandleStopBackground stops a background process.
func HandleStopBackground(ctx Context) models.ToolResponse {
	pid := models.ArgInt(ctx.Req.Args, "pid", 0)
	if pid == 0 {
		return models.ErrorResponse("stop_background: missing 'pid' argument")
	}

	if err := processTracker.StopProcess(pid); err != nil {
		return models.ErrorResponsef("stop_background: %v", err)
	}
	return models.SuccessResponse(fmt.Sprintf("stopped process %d", pid))
}

// HandleLogsProcess reads logs from a background process.
func HandleLogsProcess(ctx Context) models.ToolResponse {
	pid := models.ArgInt(ctx.Req.Args, "pid", 0)
	if pid == 0 {
		return models.ErrorResponse("logs_process: missing 'pid' argument")
	}

	logs, err := processTracker.ProcessLogs(pid)
	if err != nil {
		return models.ErrorResponsef("logs_process: %v", err)
	}
	return models.SuccessResponse(logs)
}

// HandleWaitPort waits for a TCP port to become available.
func HandleWaitPort(ctx Context) models.ToolResponse {
	port := models.ArgInt(ctx.Req.Args, "port", 0)
	if port == 0 {
		return models.ErrorResponse("wait_port: missing 'port' argument")
	}

	host := models.ArgString(ctx.Req.Args, "host")
	if host == "" {
		host = "localhost"
	}
	timeoutSec := models.ArgInt(ctx.Req.Args, "timeout", 30)
	timeout := time.Duration(timeoutSec) * time.Second

	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return models.SuccessResponse(fmt.Sprintf("port %d is open", port))
		}
		time.Sleep(500 * time.Millisecond)
	}

	return models.ErrorResponsef("wait_port: port %d not open after %v", port, timeout)
}

func buildRunCommand(path string) string {
	ext := strings.ToLower(path)
	switch {
	case strings.HasSuffix(ext, ".py"):
		return "python3 " + path
	case strings.HasSuffix(ext, ".js"):
		return "node " + path
	case strings.HasSuffix(ext, ".ts"):
		return "npx ts-node " + path
	case strings.HasSuffix(ext, ".go"):
		return "go run " + path
	case strings.HasSuffix(ext, ".sh"), strings.HasSuffix(ext, ".bash"):
		return "bash " + path
	case strings.HasSuffix(ext, ".rb"):
		return "ruby " + path
	default:
		return ""
	}
}
