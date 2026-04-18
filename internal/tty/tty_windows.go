//go:build windows

package tty

import (
	"errors"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

func Start(cmd *exec.Cmd) (*os.File, tea.Cmd, error) {
	return nil, nil, errors.New("pty-backed terminal tool is not supported on Windows")
}
