package syntax

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	chromaLexers "github.com/alecthomas/chroma/v2/lexers"
	chromaStyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	lineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func Highlight(code, filename string) string {
	lexer := chromaLexers.Match(filename)
	if lexer == nil {
		lexer = chromaLexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := chromaStyles.Get("dracula")
	if style == nil {
		style = chromaStyles.Fallback
	}

	f := formatters.Get("terminal256")
	if f == nil {
		return code
	}

	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var buf bytes.Buffer
	if err := f.Format(&buf, style, it); err != nil {
		return code
	}
	return buf.String()
}

func DiffView(oldContent, newContent, filename string) string {
	if strings.TrimSpace(newContent) == "" {
		return ""
	}
	if oldContent == "" {
		return allAdded(newContent)
	}
	return changedLines(oldContent, newContent)
}

func allAdded(content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	var sb strings.Builder
	for i, line := range lines {
		num := lineNumStyle.Render(fmt.Sprintf("%4d", i+1))
		sep := sepStyle.Render("│")
		sb.WriteString(addedStyle.Render("+") + " " + num + " " + sep + " " + addedStyle.Render(line) + "\n")
	}
	return sb.String()
}

type lineKind int

const (
	lineUnchanged lineKind = iota
	lineAdded
	lineRemoved
)

type diffLine struct {
	kind    lineKind
	content string
}

func computeLCSDiff(old, neu []string) []diffLine {
	m, n := len(old), len(neu)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if old[i-1] == neu[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var result []diffLine
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && old[i-1] == neu[j-1]:
			result = append([]diffLine{{lineUnchanged, old[i-1]}}, result...)
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			result = append([]diffLine{{lineAdded, neu[j-1]}}, result...)
			j--
		default:
			result = append([]diffLine{{lineRemoved, old[i-1]}}, result...)
			i--
		}
	}
	return result
}

func changedLines(oldContent, newContent string) string {
	old := strings.Split(strings.TrimSuffix(oldContent, "\n"), "\n")
	neu := strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
	diffs := computeLCSDiff(old, neu)

	var sb strings.Builder
	newLineNum := 1
	for _, dl := range diffs {
		sep := sepStyle.Render("│")
		switch dl.kind {
		case lineUnchanged:
			num := lineNumStyle.Render(fmt.Sprintf("%4d", newLineNum))
			sb.WriteString("  " + num + " " + sep + " " + dl.content + "\n")
			newLineNum++
		case lineAdded:
			num := lineNumStyle.Render(fmt.Sprintf("%4d", newLineNum))
			sb.WriteString(addedStyle.Render("+") + " " + num + " " + sep + " " + addedStyle.Render(dl.content) + "\n")
			newLineNum++
		case lineRemoved:
			sb.WriteString(removedStyle.Render("-") + "      " + sep + " " + removedStyle.Render(dl.content) + "\n")
		}
	}
	return sb.String()
}
