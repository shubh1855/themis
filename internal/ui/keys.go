package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Submit      key.Binding
	Quit        key.Binding
	NextSug     key.Binding
	PrevSug     key.Binding
	ScrollUp    key.Binding
	ScrollDown  key.Binding
	AllowOnce   key.Binding
	AllowAlways key.Binding
	Deny        key.Binding
}

var Keys = KeyMap{
	Submit:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
	Quit:        key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("ctrl+c", "quit")),
	NextSug:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next suggestion")),
	PrevSug:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "prev")),
	ScrollUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
	ScrollDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
	AllowOnce:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "allow once")),
	AllowAlways: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "always")),
	Deny:        key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n", "deny")),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.NextSug, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Submit, k.Quit},
		{k.NextSug, k.PrevSug},
		{k.ScrollUp, k.ScrollDown},
	}
}
