// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"namespacelabs.dev/foundation/internal/cli/inputs"
	"namespacelabs.dev/foundation/schema"
)

type stage int

const (
	NAME stage = iota
	FRAMEWORK
	FINAL
)

type model struct {
	typ string

	stage     stage
	name      textinput.Model
	framework list.Model
}

func computeModel(typ string, defaultName string) model {
	name := inputs.NewTextInput()
	name.Focus()
	name.Placeholder = defaultName
	name.CharLimit = 32

	framework := inputs.NewFrameworkInput([]schema.Framework{
		schema.Framework_GO_GRPC,
		schema.Framework_WEB,
		schema.Framework_NODEJS,
	})

	m, err := tea.NewProgram(model{
		typ:       typ,
		name:      name,
		framework: framework,
	}).StartReturningModel()
	if err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}

	return m.(model)
}

func (m model) IsFinal() bool {
	return m.stage == FINAL
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (_ tea.Model, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.framework.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.stage == NAME && m.name.Value() == "" {
				m.name.SetValue(m.name.Placeholder)
			}

			m.stage++

			if m.stage == FINAL {
				return m, tea.Quit
			}

			return m, nil
		}
	}

	switch m.stage {
	case NAME:
		m.name, cmd = m.name.Update(msg)
	case FRAMEWORK:
		m.framework, cmd = m.framework.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	var b strings.Builder

	switch m.stage {
	case NAME:
		fmt.Fprintf(&b, "What's the name of your %s?\n\n%s", m.typ, m.name.View())
	case FRAMEWORK:
		fmt.Fprintf(&b, m.framework.View())
	}

	fmt.Fprintf(&b, "\n\nPress enter to confirm.")

	return b.String()
}
