// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	"namespacelabs.dev/foundation/internal/console"
)

func Ask(ctx context.Context, title, description, placeholder string) (string, error) {
	done := console.EnterInputMode(ctx)
	defer done()

	p := tea.NewProgram(initialAskModel(title, description, placeholder))

	final, err := p.StartReturningModel()
	if err != nil {
		return "", err
	}

	if final.(askModel).canceled {
		return "", nil
	}

	return final.(askModel).textInput.Value(), nil
}

type askModel struct {
	title, description string
	textInput          textinput.Model
	help               help.Model
	wordwrap           bool
	canceled           bool
}

func initialAskModel(title, description, placeholder string) askModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = 64

	help := help.New()

	return askModel{
		title:       title,
		description: description,
		textInput:   ti,
		help:        help,
		wordwrap:    true,
	}
}

func (m askModel) Init() tea.Cmd {
	return nil
}

func (m askModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, tea.Quit

		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.wordwrap = msg.Width > 80
		m.textInput.Width = msg.Width - 5
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m askModel) View() string {
	body := fmt.Sprintf("%s\n\n%s\n", titleStyle.Render(m.title), descStyle.Render(m.description))
	if m.wordwrap {
		body = wordwrap.String(body, 80)
	}

	return askMainStyle.Render(fmt.Sprintf("%s\n%s\n\n%s", body, m.textInput.View(), m.help.View(escOnly{})))
}

type escOnly struct{}

func (escOnly) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "quit"),
		),
	}
}

func (escOnly) FullHelp() [][]key.Binding { return nil }
