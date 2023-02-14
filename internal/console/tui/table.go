// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func StaticTable(ctx context.Context, cols []table.Column, rows []table.Row) error {
	_, err := Table(ctx, cols, rows, false)
	return err
}

func SelectTable(ctx context.Context, cols []table.Column, rows []table.Row) (table.Row, error) {
	return Table(ctx, cols, rows, true)
}

func Table(ctx context.Context, cols []table.Column, rows []table.Row, selectRow bool) (table.Row, error) {
	height := 2 + len(rows)
	if height > maxHeight {
		height = maxHeight
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	if !selectRow {
		s.Selected = s.Selected.
			Foreground(s.Cell.GetForeground()).
			Background(s.Cell.GetBackground()).
			Bold(false)
	}
	t.SetStyles(s)

	m := tableModel{
		table:     t,
		maxHeight: height,
		selectRow: selectRow,
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}

	return final.(tableModel).selectedRow, nil
}

type tableModel struct {
	table       table.Model
	maxHeight   int
	selectRow   bool
	selectedRow table.Row
}

func (t tableModel) Init() tea.Cmd {
	if t.selectRow {
		return nil
	}
	return tea.Quit
}

func (t tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !t.selectRow {
		return t, tea.Quit
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			t.selectedRow = t.table.SelectedRow()
			return t, tea.Quit

		case tea.KeyCtrlC, tea.KeyEsc:
			return t, tea.Quit
		}
	case tea.WindowSizeMsg:
		t.table.SetWidth(msg.Width)
		if msg.Height > t.maxHeight {
			t.table.SetHeight(t.maxHeight)
		} else {
			t.table.SetHeight(msg.Height)
		}
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

func (t tableModel) View() string {
	return tableMainStyle.Render(t.table.View()) + "\n"
}
