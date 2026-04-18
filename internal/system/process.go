package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// ProcessTracker manages background processes started by the agent.
type ProcessTracker struct {
	mu    sync.Mutex
	procs map[int]*BackgroundProcess
}

// BackgroundProcess represents a running background process.
type BackgroundProcess struct {
	PID     int
	Cmd     *exec.Cmd
	Cancel  context.CancelFunc
	LogFile string
}

// NewProcessTracker creates a new process tracker.
func NewProcessTracker() *ProcessTracker {
	return &ProcessTracker{
		procs: make(map[int]*BackgroundProcess),
	}
}

// StartBackground starts a command in the background and tracks it.
func (pt *ProcessTracker) StartBackground(ctx context.Context, command string, args []string, dir string) (*models.ProcessResult, error) {
	ctx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	// Create a log file for output
	logFile := fmt.Sprintf("/tmp/agent_proc_%d.log", os.Getpid())
	f, err := os.Create(logFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("system: create log: %w", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		cancel()
		f.Close()
		return nil, fmt.Errorf("system: start background: %w", err)
	}

	pid := cmd.Process.Pid

	pt.mu.Lock()
	pt.procs[pid] = &BackgroundProcess{
		PID:     pid,
		Cmd:     cmd,
		Cancel:  cancel,
		LogFile: logFile,
	}
	pt.mu.Unlock()

	// Wait in background to clean up
	go func() {
		cmd.Wait()
		f.Close()
	}()

	return &models.ProcessResult{PID: pid}, nil
}

// StopProcess stops a tracked background process.
func (pt *ProcessTracker) StopProcess(pid int) error {
	pt.mu.Lock()
	bp, ok := pt.procs[pid]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("system: no tracked process with PID %d", pid)
	}
	delete(pt.procs, pid)
	pt.mu.Unlock()

	bp.Cancel()
	return nil
}

// ProcessLogs reads the log file for a tracked background process.
func (pt *ProcessTracker) ProcessLogs(pid int) (string, error) {
	pt.mu.Lock()
	bp, ok := pt.procs[pid]
	pt.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("system: no tracked process with PID %d", pid)
	}

	data, err := os.ReadFile(bp.LogFile)
	if err != nil {
		return "", fmt.Errorf("system: read logs: %w", err)
	}

	return truncateOutput(string(data)), nil
}

// ListProcesses returns all tracked PIDs.
func (pt *ProcessTracker) ListProcesses() []int {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pids := make([]int, 0, len(pt.procs))
	for pid := range pt.procs {
		pids = append(pids, pid)
	}
	return pids
}
