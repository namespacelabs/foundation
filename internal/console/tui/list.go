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

const maxHeight = 20

func Select[V list.DefaultItem](ctx context.Context, title string, items []V) (list.DefaultItem, error) {
	done := console.EnterInputMode(ctx)
	defer done()

	downcast := make([]list.Item, len(items))
	for k, item := range items {
		downcast[k] = item
	}

	height := 5 + len(items)*3
	if height > maxHeight {
		height = maxHeight
	}

	p := tea.NewProgram(initialSelectModel(title, downcast, height))

	final, err := p.StartReturningModel()
	if err != nil {
		return nil, err
	}

	return final.(selectModel).selected, nil
}

type selectModel struct {
	maxHeight int
	list      list.Model
	selected  list.DefaultItem
}

func initialSelectModel(title string, items []list.Item, height int) selectModel {
	li := list.New(items, itemDelegate{list.NewDefaultItemStyles()}, 40, height)
	li.Title = title
	li.SetShowStatusBar(false)
	li.SetFilteringEnabled(false)
	li.Styles.Title = titleStyle
	li.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle
	li.Styles.HelpStyle = list.DefaultStyles().HelpStyle

	return selectModel{maxHeight: height, list: li}
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
		if msg.Height > m.maxHeight {
			m.list.SetHeight(m.maxHeight)
		} else {
			m.list.SetHeight(msg.Height)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() string {
	return mainStyle.Render(m.list.View())
}

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
