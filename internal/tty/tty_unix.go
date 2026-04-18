//go:build !windows

package tty

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
)

func Start(cmd *exec.Cmd) (*os.File, tea.Cmd, error) {
	master, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}
	return master, ReadOutput(master), nil
}
