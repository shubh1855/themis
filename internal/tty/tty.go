package tty

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type OutputMsg string

type DoneMsg struct{ Err error }

func ReadOutput(master *os.File) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := master.Read(buf)
		if n > 0 {
			return OutputMsg(cleanOutput(string(buf[:n])))
		}
		if err != nil {
			return DoneMsg{Err: err}
		}
		return ReadOutput(master)()
	}
}

func KeyToBytes(k string) []byte {
	switch k {
	case "enter":
		return []byte("\r")
	case "backspace":
		return []byte("\x7f")
	case "ctrl+c":
		return []byte("\x03")
	case "ctrl+d":
		return []byte("\x04")
	case "ctrl+z":
		return []byte("\x1a")
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "tab":
		return []byte("\t")
	case "esc":
		return []byte("\x1b")
	case "space":
		return []byte(" ")
	default:
		return []byte(k)
	}
}

func cleanOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] == ';' || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			if j < len(s) {
				final := s[j]
				if final == 'm' {
					out.WriteString(s[i : j+1])
				}
				i = j + 1
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
