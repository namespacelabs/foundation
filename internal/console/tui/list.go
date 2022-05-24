// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tui

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"namespacelabs.dev/foundation/internal/console"
)

func Select[V list.DefaultItem](ctx context.Context, title string, items []V) (list.DefaultItem, error) {
	done := console.EnterInputMode(ctx)
	defer done()

	downcast := make([]list.Item, len(items))
	for k, item := range items {
		downcast[k] = item
	}

	p := tea.NewProgram(initialSelectModel(title, downcast))

	final, err := p.StartReturningModel()
	if err != nil {
		return nil, err
	}

	return final.(selectModel).selected, nil
}

type selectModel struct {
	list     list.Model
	selected list.DefaultItem
}

func initialSelectModel(title string, items []list.Item) selectModel {
	li := list.New(items, itemDelegate{list.NewDefaultItemStyles()}, 20, 14)
	li.Title = title
	li.SetShowStatusBar(false)
	li.SetFilteringEnabled(false)
	li.Styles.Title = lipgloss.NewStyle().Bold(true)
	li.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle
	li.Styles.HelpStyle = list.DefaultStyles().HelpStyle

	return selectModel{list: li}
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if selected, ok := m.list.SelectedItem().(list.DefaultItem); ok {
				m.selected = selected
			}
			return m, tea.Quit

		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		if msg.Height > 14 {
			m.list.SetHeight(14)
		} else {
			m.list.SetHeight(msg.Height)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() string {
	return selectMainStyle.Render(m.list.View())
}

var (
	selectMainStyle = lipgloss.NewStyle().Margin(0, 2, 1)
)

type itemDelegate struct {
	styles list.DefaultItemStyles
}

func (d itemDelegate) Height() int                               { return 2 }
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(list.DefaultItem)
	if !ok {
		return
	}

	var titleStyle, descStyle lipgloss.Style
	if index == m.Index() {
		titleStyle = d.styles.SelectedTitle
		descStyle = d.styles.SelectedDesc
	} else {
		titleStyle = d.styles.NormalTitle
		descStyle = d.styles.NormalDesc
	}

	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(item.Title()), descStyle.Render(item.Description()))
}
