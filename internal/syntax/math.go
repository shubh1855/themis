package syntax

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var mathSymbols = strings.NewReplacer(
	`\alpha`, "α", `\beta`, "β", `\gamma`, "γ", `\delta`, "δ",
	`\epsilon`, "ε", `\zeta`, "ζ", `\eta`, "η", `\theta`, "θ",
	`\iota`, "ι", `\kappa`, "κ", `\lambda`, "λ", `\mu`, "μ",
	`\nu`, "ν", `\xi`, "ξ", `\pi`, "π", `\rho`, "ρ",
	`\sigma`, "σ", `\tau`, "τ", `\upsilon`, "υ", `\phi`, "φ",
	`\chi`, "χ", `\psi`, "ψ", `\omega`, "ω",
	`\Gamma`, "Γ", `\Delta`, "Δ", `\Theta`, "Θ", `\Lambda`, "Λ",
	`\Xi`, "Ξ", `\Pi`, "Π", `\Sigma`, "Σ", `\Phi`, "Φ", `\Psi`, "Ψ", `\Omega`, "Ω",
	`\sum`, "∑", `\prod`, "∏", `\int`, "∫", `\oint`, "∮",
	`\infty`, "∞", `\partial`, "∂", `\nabla`, "∇",
	`\forall`, "∀", `\exists`, "∃", `\nexists`, "∄",
	`\in`, "∈", `\notin`, "∉", `\subset`, "⊂", `\supset`, "⊃",
	`\cup`, "∪", `\cap`, "∩", `\emptyset`, "∅",
	`\leq`, "≤", `\geq`, "≥", `\neq`, "≠", `\approx`, "≈",
	`\equiv`, "≡", `\sim`, "∼", `\propto`, "∝",
	`\rightarrow`, "→", `\leftarrow`, "←", `\leftrightarrow`, "↔",
	`\Rightarrow`, "⇒", `\Leftarrow`, "⇐", `\Leftrightarrow`, "⇔",
	`\cdot`, "·", `\times`, "×", `\div`, "÷", `\pm`, "±", `\mp`, "∓",
	`\sqrt`, "√", `\ldots`, "…", `\cdots`, "⋯",
	`\langle`, "⟨", `\rangle`, "⟩",
)

var (
	inlineMathRe = regexp.MustCompile(`\$([^$\n]+?)\$`)
	blockMathRe  = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`)
	fracRe       = regexp.MustCompile(`\\frac\{([^}]+)\}\{([^}]+)\}`)
	supRe        = regexp.MustCompile(`\^(\{[^}]+\}|[A-Za-z0-9])`)
	subRe        = regexp.MustCompile(`_(\{[^}]+\}|[A-Za-z0-9])`)
	curlyRe      = regexp.MustCompile(`[{}]`)

	mathInlineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	mathBlockStyle  = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Bold(true).
			PaddingLeft(4).
			PaddingTop(0)
)

func convertMathExpr(expr string) string {
	s := fracRe.ReplaceAllStringFunc(expr, func(m string) string {
		parts := fracRe.FindStringSubmatch(m)
		if len(parts) == 3 {
			return "(" + parts[1] + "/" + parts[2] + ")"
		}
		return m
	})
	s = supRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.Trim(supRe.FindStringSubmatch(m)[1], "{}")
		sup := toSuperscript(inner)
		if sup != "" {
			return sup
		}
		return "^" + inner
	})
	s = subRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.Trim(subRe.FindStringSubmatch(m)[1], "{}")
		sub := toSubscript(inner)
		if sub != "" {
			return sub
		}
		return "_" + inner
	})
	s = mathSymbols.Replace(s)
	s = curlyRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func toSuperscript(s string) string {
	sup := map[rune]string{
		'0': "⁰", '1': "¹", '2': "²", '3': "³", '4': "⁴",
		'5': "⁵", '6': "⁶", '7': "⁷", '8': "⁸", '9': "⁹",
		'n': "ⁿ", 'i': "ⁱ", '+': "⁺", '-': "⁻", '=': "⁼",
		'(': "⁽", ')': "⁾",
	}
	var b strings.Builder
	for _, r := range s {
		v, ok := sup[r]
		if !ok {
			return ""
		}
		b.WriteString(v)
	}
	return b.String()
}

func toSubscript(s string) string {
	sub := map[rune]string{
		'0': "₀", '1': "₁", '2': "₂", '3': "₃", '4': "₄",
		'5': "₅", '6': "₆", '7': "₇", '8': "₈", '9': "₉",
		'n': "ₙ", 'i': "ᵢ", 'j': "ⱼ", 'k': "ₖ", '+': "₊", '-': "₋",
	}
	var b strings.Builder
	for _, r := range s {
		v, ok := sub[r]
		if !ok {
			return ""
		}
		b.WriteString(v)
	}
	return b.String()
}

// RenderMath replaces $...$ and $$...$$ LaTeX expressions with styled Unicode.
func RenderMath(text string) string {
	// Block math first (before inline, to avoid double-processing)
	text = blockMathRe.ReplaceAllStringFunc(text, func(m string) string {
		inner := blockMathRe.FindStringSubmatch(m)[1]
		rendered := convertMathExpr(strings.TrimSpace(inner))
		return "\n" + mathBlockStyle.Render("∫ "+rendered) + "\n"
	})
	text = inlineMathRe.ReplaceAllStringFunc(text, func(m string) string {
		inner := inlineMathRe.FindStringSubmatch(m)[1]
		rendered := convertMathExpr(inner)
		return mathInlineStyle.Render(rendered)
	})
	return text
}

// HasMath reports whether text contains LaTeX math expressions.
func HasMath(text string) bool {
	return inlineMathRe.MatchString(text) || blockMathRe.MatchString(text)
}
