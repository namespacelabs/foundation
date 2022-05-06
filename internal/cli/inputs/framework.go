// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package inputs

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"namespacelabs.dev/foundation/schema"
)

type item struct {
	framework schema.Framework
}

func (i item) FilterValue() string { return i.framework.String() }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	str := fmt.Sprintf("%d. %s", index+1, listItem.(item).framework.String())

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render("> " + s)
		}
	}

	fmt.Fprintf(w, fn(str))
}

func NewFrameworkInput(frameworks []schema.Framework) list.Model {
	var items []list.Item
	for _, fmwk := range frameworks {
		items = append(items, item{framework: fmwk})
	}

	input := list.New(items, itemDelegate{}, 0, 6+len(items))
	input.Title = "Which framework do you want to use?"
	input.SetShowStatusBar(false)
	input.SetFilteringEnabled(false)
	input.Styles.Title = titleStyle
	input.Styles.PaginationStyle = paginationStyle
	input.Styles.HelpStyle = helpStyle

	return input
}

func SelectedFramework(m list.Model) (schema.Framework, error) {
	item, ok := m.SelectedItem().(item)
	if !ok {
		return schema.Framework_FRAMEWORK_UNSPECIFIED, fmt.Errorf("list is not a framework list")
	}

	return item.framework, nil
}
